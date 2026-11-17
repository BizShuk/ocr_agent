# Receipt Pipeline Hardening Implementation Plan

> `For agentic workers:` REQUIRED SUB-SKILL: Use `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

`Goal:` Harden receipt classification and archival so untrusted model output cannot overwrite or misplace records, failures are retryable and observable, and pm2 has one sequential pipeline entrypoint.

`Architecture:` Shared validation, naming, collision, HTTP, and failure-safe archive primitives live in `receipt_common.py`. Category scripts provide prompts and category-specific filename builders; a small orchestrator runs all three stages sequentially and reports an aggregated exit status.

`Tech Stack:` Python 3 standard library (`argparse`, `datetime`, `decimal`, `json`, `pathlib`, `tempfile`, `unittest`, `unittest.mock`), Ollama HTTP API, pm2 ecosystem configuration.

## Global Constraints

- Preserve `receipts/{general,medical}/<YYYY-MM>/<date>_<name>_<currency><amount>.<ext>` for new files.
- Format new monetary filename components with exactly two decimal places.
- Do not rename, migrate, read, move, or transmit existing files under `receipts/` during implementation or verification.
- Dry-run must perform zero filesystem writes.
- Do not execute any receipt script with `--apply` against project data.
- Do not start, restart, reload, save, pause, or stop pm2.
- Use only the Python standard library; do not add package management.

---

### Task 1: Shared validation and naming primitives

`Files:`

- Create: `tests/__init__.py`
- Create: `tests/test_receipt_common.py`
- Modify: `scripts/receipt/receipt_common.py`

`Interfaces:`

- Produces: `ReceiptValidationError`, `require_mapping(value)`, `validate_date(value, today=None)`, `normalize_currency(value)`, `format_amount(value)`, `sanitize_name(value)`, `choose_destination(dest_dir, new_name)`, `ensure_within(root, path)`, and `query_ollama(prompt, image_b64, model, ollama_url)`.
- Preserves: `ask_ollama(...) -> dict | None` as the JSON wrapper used by existing scripts.

- [x] `Step 1: Write failing validation tests`

```python
class ValidationTests(unittest.TestCase):
    def test_rejects_non_mapping_model_response(self):
        with self.assertRaises(ReceiptValidationError):
            require_mapping([])

    def test_rejects_invalid_future_and_traversal_dates(self):
        for value in ("2026-02-30", "2026/07/18", "../../tmp", "2026-07-19"):
            with self.subTest(value=value), self.assertRaises(ReceiptValidationError):
                validate_date(value, today=date(2026, 7, 18))

    def test_normalizes_currency_and_amount(self):
        self.assertEqual("sgd", normalize_currency("SGD"))
        self.assertEqual("12.50", format_amount(12.5))

    def test_rejects_unsafe_currency_and_amount(self):
        for value in ("../", "sg/d", "SGDX"):
            with self.assertRaises(ReceiptValidationError):
                normalize_currency(value)
        for value in ("NaN", "Infinity", -1, "12.345"):
            with self.assertRaises(ReceiptValidationError):
                format_amount(value)
```

- [x] `Step 2: Run tests and confirm RED`

Run: `python3 -m unittest tests.test_receipt_common -v`

Expected: import failure because the new validation interfaces do not exist.

- [x] `Step 3: Implement minimal validation and HTTP primitives`

```python
class ReceiptValidationError(ValueError):
    pass


def validate_date(value, today=None):
    if not isinstance(value, str) or not re.fullmatch(r"\d{4}-\d{2}-\d{2}", value):
        raise ReceiptValidationError("date must use YYYY-MM-DD")
    try:
        parsed = date.fromisoformat(value)
    except ValueError as exc:
        raise ReceiptValidationError("date is not a calendar date") from exc
    if parsed > (today or date.today()):
        raise ReceiptValidationError("date cannot be in the future")
    return value


def format_amount(value):
    try:
        amount = Decimal(str(value))
    except (InvalidOperation, ValueError) as exc:
        raise ReceiptValidationError("amount must be numeric") from exc
    if not amount.is_finite() or amount < 0 or amount.as_tuple().exponent < -2:
        raise ReceiptValidationError("amount must be finite, non-negative, and use at most two decimals")
    return f"{amount:.2f}"
```

Implement the remaining declared interfaces with exact type and containment checks. Split raw HTTP transport into `query_ollama`; keep `ask_ollama` responsible for JSON parsing and mapping validation.

- [x] `Step 4: Run tests and confirm GREEN`

Run: `python3 -m unittest tests.test_receipt_common -v`

Expected: all Task 1 tests pass.

- [x] `Step 5: Commit Task 1`

```bash
git add tests/__init__.py tests/test_receipt_common.py scripts/receipt/receipt_common.py
git commit -m "fix: validate receipt model output"
```

### Task 2: Failure-safe archive processing and medical contract

`Files:`

- Modify: `tests/test_receipt_common.py`
- Create: `tests/test_receipt_processors.py`
- Modify: `scripts/receipt/receipt_common.py`
- Modify: `scripts/receipt/process_general_receipts.py`
- Modify: `scripts/receipt/process_medical_receipts.py`

`Interfaces:`

- Consumes: Task 1 validation and naming helpers.
- Produces: `archive_receipt(source, dest_path, json_path, info)` and `ReceiptProcessor.run(argv=None) -> int`.
- Category builders continue to expose `build_new_name(info, original_ext) -> str` but validate every component and format amounts to two decimals.

- [x] `Step 1: Write failing archive and processor tests`

```python
def test_archive_rolls_image_back_when_sidecar_commit_fails(self):
    with TemporaryDirectory() as tmp:
        root = Path(tmp)
        source = root / "tmp" / "source.jpg"
        source.parent.mkdir()
        source.write_bytes(b"image")
        dest = root / "2026-07" / "receipt.jpg"
        sidecar = dest.with_suffix(".json")
        with mock.patch("receipt_common.os.replace", side_effect=OSError("disk failure")):
            with self.assertRaises(OSError):
                archive_receipt(source, dest, sidecar, {"date": "2026-07-18"})
        self.assertTrue(source.exists())
        self.assertFalse(dest.exists())
        self.assertFalse(sidecar.exists())

def test_non_mc_requires_currency_and_total_amount(self):
    with self.assertRaises(ReceiptValidationError):
        build_medical_name({
            "date": "2026-07-18",
            "clinic_name": "clinic",
            "is_medical_certificate": False,
        }, ".jpg")

def test_general_amount_has_two_decimals(self):
    name = build_general_name({
        "date": "2026-07-18",
        "merchant": "shop",
        "currency": "sgd",
        "total_amount": 12.5,
    }, ".jpg")
    self.assertEqual("2026-07-18_shop_sgd12.50.jpg", name)
```

Add a processor test using temporary directories and patched `prepare_image`/`ask_ollama`: the first file raises an I/O error, the second succeeds, and `run([])` returns `1` after attempting both.

- [x] `Step 2: Run tests and confirm RED`

Run: `python3 -m unittest tests.test_receipt_common tests.test_receipt_processors -v`

Expected: failures for missing `archive_receipt`, unsafe builders, and current batch-abort behavior.

- [x] `Step 3: Implement failure-safe archive and per-file isolation`

```python
def archive_receipt(source, dest_path, json_path, info):
    dest_path.parent.mkdir(parents=True, exist_ok=True)
    fd, temp_name = tempfile.mkstemp(prefix=f".{json_path.stem}.", suffix=".tmp", dir=dest_path.parent)
    temp_path = Path(temp_name)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(info, handle, ensure_ascii=False, indent=2)
            handle.flush()
            os.fsync(handle.fileno())
        source.rename(dest_path)
        try:
            os.replace(temp_path, json_path)
        except Exception:
            dest_path.rename(source)
            raise
    finally:
        temp_path.unlink(missing_ok=True)
```

Update `ReceiptProcessor.run(argv=None)` so each file has one `try/except/finally`, temporary HEIC files are always cleaned, validation failures count as errors, destination selection checks both image and sidecar, and the return value is `1` when `errors` is non-empty.

- [x] `Step 4: Run tests and confirm GREEN`

Run: `python3 -m unittest tests.test_receipt_common tests.test_receipt_processors -v`

Expected: all Task 1 and Task 2 tests pass.

- [x] `Step 5: Commit Task 2`

```bash
git add tests/test_receipt_common.py tests/test_receipt_processors.py scripts/receipt/receipt_common.py scripts/receipt/process_general_receipts.py scripts/receipt/process_medical_receipts.py
git commit -m "fix: make receipt archival failure safe"
```

### Task 3: Classifier and analyzer CLI safety

`Files:`

- Create: `tests/test_receipt_cli.py`
- Modify: `scripts/receipt/classify_receipts.py`
- Modify: `scripts/receipt/analyze_receipt.py`

`Interfaces:`

- Consumes: `choose_destination`, `prepare_image`, `ask_ollama`, and `query_ollama` from `receipt_common.py`.
- Produces: `classify_receipts.main(argv=None) -> int` and `analyze_receipt.main(argv=None) -> int`.

- [x] `Step 1: Write failing CLI tests`

```python
def test_classifier_dry_run_creates_no_directories(self):
    with TemporaryDirectory() as tmp:
        root = Path(tmp)
        entry = root / "receipts" / "tmp"
        entry.mkdir(parents=True)
        (entry / "IMG.JPG").write_bytes(b"image")
        with patched_classifier_paths(root), mock.patch(
            "classify_receipts.prepare_image", return_value=("base64", None)
        ), mock.patch(
            "classify_receipts.ask_ollama", return_value={"category": "general_receipt"}
        ):
            self.assertEqual(0, classify_receipts.main([]))
        self.assertFalse((root / "receipts" / "general").exists())

def test_classifier_uses_suffix_instead_of_overwriting(self):
    with TemporaryDirectory() as tmp:
        root = Path(tmp)
        entry = root / "receipts" / "tmp"
        general = root / "receipts" / "general" / "tmp"
        medical = root / "receipts" / "medical" / "tmp"
        entry.mkdir(parents=True)
        general.mkdir(parents=True)
        (entry / "IMG.JPG").write_bytes(b"new")
        (general / "IMG.JPG").write_bytes(b"old")
        with mock.patch.multiple(
            classify_receipts,
            ENTRY_TMP_DIR=entry,
            GENERAL_TMP_DIR=general,
            MEDICAL_TMP_DIR=medical,
        ), mock.patch(
            "classify_receipts.prepare_image", return_value=("base64", None)
        ), mock.patch(
            "classify_receipts.ask_ollama", return_value={"category": "general_receipt"}
        ):
            self.assertEqual(0, classify_receipts.main(["--apply"]))
        self.assertEqual(b"old", (general / "IMG.JPG").read_bytes())
        self.assertEqual(b"new", (general / "IMG_2.JPG").read_bytes())

def test_analyzer_uses_repository_root_and_http_transport(self):
    expected_root = Path(analyze_receipt.__file__).resolve().parent.parent.parent
    self.assertEqual(expected_root, analyze_receipt.PROJECT_DIR)
    with TemporaryDirectory() as tmp:
        image = Path(tmp) / "receipt.jpg"
        image.write_bytes(b"image")
        with mock.patch(
            "analyze_receipt.prepare_image", return_value=("base64", None)
        ), mock.patch(
            "analyze_receipt.query_ollama", return_value="raw response"
        ) as query:
            self.assertEqual("raw response", analyze_receipt.ask(image))
        query.assert_called_once_with(
            analyze_receipt.PROMPT,
            "base64",
            analyze_receipt.MODEL,
            analyze_receipt.OLLAMA_URL,
        )
```

Add analyzer tests asserting `PROJECT_DIR` equals the repository root and `ask()` calls `query_ollama` with base64 in the HTTP payload interface rather than spawning `curl`.

- [x] `Step 2: Run tests and confirm RED`

Run: `python3 -m unittest tests.test_receipt_cli -v`

Expected: classifier dry-run creates directories, collision overwrites, and analyzer project root is wrong.

- [x] `Step 3: Implement CLI safety`

Move classifier directory creation inside the `--apply` branch, select a unique destination before `rename`, isolate per-file errors, and return a non-zero status when any error occurs:

```python
if category == "general_receipt":
    dest_dir = GENERAL_TMP_DIR
elif category == "medical_receipt":
    dest_dir = MEDICAL_TMP_DIR
else:
    skipped += 1
    continue

dest = choose_destination(dest_dir, f.name)[0]
if args.apply:
    dest_dir.mkdir(parents=True, exist_ok=True)
    f.rename(dest)

return 1 if errors else 0
```

Refactor analyzer to use the common HTTP body transport and guaranteed temporary cleanup:

```python
PROJECT_DIR = Path(__file__).resolve().parent.parent.parent


def ask(image_path):
    temp = None
    try:
        image_b64, temp = prepare_image(image_path)
        return query_ollama(PROMPT, image_b64, MODEL, OLLAMA_URL).strip()
    finally:
        if temp:
            temp.unlink(missing_ok=True)
```

- [x] `Step 4: Run tests and confirm GREEN`

Run: `python3 -m unittest tests.test_receipt_cli -v`

Expected: all Task 3 tests pass without reading project receipt files.

- [x] `Step 5: Commit Task 3`

```bash
git add tests/test_receipt_cli.py scripts/receipt/classify_receipts.py scripts/receipt/analyze_receipt.py
git commit -m "fix: protect receipt classification and analysis"
```

### Task 4: Sequential pipeline and pm2 schedule

`Files:`

- Create: `tests/test_receipt_pipeline.py`
- Create: `scripts/receipt/process_receipts.py`
- Modify: `ecosystem.config.js`
- Modify: `.vscode/tasks.json`

`Interfaces:`

- Produces: `process_receipts.main(argv=None) -> int` and `STAGES`, an ordered tuple of the three script filenames.
- Replaces: the standalone pm2 classifier entry with the single `process-receipts` pipeline entry.

- [x] `Step 1: Write failing orchestrator tests`

```python
def test_runs_all_stages_in_order_and_aggregates_failure(self):
    results = [CompletedProcess([], 1), CompletedProcess([], 0), CompletedProcess([], 0)]
    with mock.patch("process_receipts.subprocess.run", side_effect=results) as run:
        self.assertEqual(1, process_receipts.main(["--apply"]))
    self.assertEqual(3, run.call_count)
    self.assertEqual(
        ["classify_receipts.py", "process_general_receipts.py", "process_medical_receipts.py"],
        [Path(call.args[0][1]).name for call in run.call_args_list],
    )
    self.assertTrue(all("--apply" in call.args[0] for call in run.call_args_list))
```

Add a test that three zero return codes produce pipeline status `0`.

- [x] `Step 2: Run tests and confirm RED`

Run: `python3 -m unittest tests.test_receipt_pipeline -v`

Expected: import failure because `process_receipts.py` does not exist.

- [x] `Step 3: Implement orchestrator and configuration`

```python
STAGES = (
    "classify_receipts.py",
    "process_general_receipts.py",
    "process_medical_receipts.py",
)


def main(argv=None):
    parser = argparse.ArgumentParser(description="Run the receipt pipeline")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args(argv)
    failed = False
    for stage in STAGES:
        command = [sys.executable, str(Path(__file__).with_name(stage))]
        if args.apply:
            command.append("--apply")
        failed = subprocess.run(command, check=False).returncode != 0 or failed
    return 1 if failed else 0
```

Configure pm2 with one `process-receipts` app, `args: ["--apply"]`, `cron: "0 5 * * *"`, and `autorestart: false`. Point the VS Code `Process All` task at the same orchestrator.

- [x] `Step 4: Run tests and syntax checks`

Run: `python3 -m unittest tests.test_receipt_pipeline -v && node --check ecosystem.config.js && python3 -m json.tool .vscode/tasks.json >/dev/null`

Expected: all pipeline tests and both configuration syntax checks pass.

- [x] `Step 5: Commit Task 4`

```bash
git add tests/test_receipt_pipeline.py scripts/receipt/process_receipts.py ecosystem.config.js .vscode/tasks.json
git commit -m "fix: schedule sequential receipt pipeline"
```

### Task 5: Documentation consistency and full verification

`Files:`

- Modify: `README.md`
- Modify: `CLAUDE.md`
- Modify: `plans/2026-07-18-receipt-pipeline-hardening.md`

`Interfaces:`

- Documents: canonical categorized receipt layout, dry-run/apply semantics, exit status, pipeline ordering, and pm2 schedule.
- Preserves: no bulk migration of existing archive files.

- [x] `Step 1: Update documentation`

Replace stale claims that the repository has no code, Git, testing, or automation. Document the category-specific underscore naming format and fixed two-decimal amounts. Replace the old standalone general-receipt cron description with the sequential `process-receipts` schedule.

- [x] `Step 2: Run consistency searches`

Run:

```bash
rg -n 'process-general-receipts|scripts/analyze_receipt.py|<source>_<date>|<來源>_<日期>|no source code|沒有原始碼|no Git repository|沒有 Git 倉庫' README.md CLAUDE.md .vscode ecosystem.config.js scripts/receipt -g '!scripts/receipt/backup/**'
```

Expected: no stale contradictions remain; any matches are explicit historical-context statements rather than current claims.

- [x] `Step 3: Run the complete isolated verification suite`

Run:

```bash
python3 -m unittest discover -s tests -v
python3 -m py_compile scripts/receipt/*.py
node --check ecosystem.config.js
python3 -m json.tool .vscode/tasks.json >/dev/null
git diff --check
```

Expected: all tests pass, Python compilation succeeds, configuration syntax is valid, and no whitespace errors are reported. No command includes `--apply` against project data.

- [x] `Step 4: Review the final diff and project state`

Run: `git diff --stat HEAD~4..HEAD && git status --short`

Expected: only planned implementation and documentation files plus the user's pre-existing `README.todo` and `assets/` changes are present.

- [x] `Step 5: Commit Task 5`

```bash
git add README.md CLAUDE.md plans/2026-07-18-receipt-pipeline-hardening.md
git commit -m "docs: align receipt pipeline documentation"
```
