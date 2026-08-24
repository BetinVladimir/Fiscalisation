#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
from pathlib import Path
from urllib import error, request

BASE_DIR = Path(__file__).resolve().parent
SRC_FILE = BASE_DIR / "Naredba N-18 2006 canonical clean.md"
CHUNK_DIR = BASE_DIR / "tmp"
OUT_DIR = BASE_DIR / "tmp_ru"
FINAL_OUT = BASE_DIR / "Naredba N-18 2006 canonical clean RU ollama.md"
MODEL = "translategemma"
OLLAMA_URL = "http://localhost:11434/api/generate"
MAX_CHARS = 1800


def split_into_chunks(text: str, max_chars: int = MAX_CHARS):
    lines = text.splitlines(keepends=True)
    chunks: list[str] = []
    current: list[str] = []
    current_len = 0

    for line in lines:
        line_len = len(line)
        if current and current_len + line_len > max_chars and current_len > 0:
            chunks.append("".join(current).rstrip() + "\n")
            current = [line]
            current_len = line_len
        else:
            current.append(line)
            current_len += line_len

    if current:
        chunks.append("".join(current).rstrip() + "\n")

    return chunks


def write_source_chunks(force: bool = False):
    CHUNK_DIR.mkdir(exist_ok=True)
    if not SRC_FILE.exists():
        raise FileNotFoundError(f"Source file not found: {SRC_FILE}")

    text = SRC_FILE.read_text(encoding="utf-8", errors="replace")
    chunks = split_into_chunks(text)

    if not force:
        existing = sorted(CHUNK_DIR.glob("*.txt"))
        if existing:
            print(f"Chunk dir already contains {len(existing)} files; skipping generation.")
            return len(existing)

    for index, chunk in enumerate(chunks, 1):
        target = CHUNK_DIR / f"{index:05d}.txt"
        target.write_text(chunk, encoding="utf-8")

    print(f"Created {len(chunks)} chunks in {CHUNK_DIR}")
    return len(chunks)


def translate_chunk(text: str):
    prompt = (
        "Translate Bulgarian to Russian. Preserve the legal and administrative style of the original text. "
        "Do not add explanations, comments, headers, or placeholder text. Output only translated text.\n\n"
        f"{text}"
    )

    payload = json.dumps(
        {
            "model": MODEL,
            "prompt": prompt,
            "stream": False,
            "system": "You are a professional legal translator. Translate Bulgarian text into Russian, preserving legal and administrative terminology and structure. Do not add commentary or explain the text. Output only the translated text.",
            "options": {"temperature": 0.1, "num_predict": 4096},
        }
    ).encode("utf-8")

    req = request.Request(OLLAMA_URL, data=payload, headers={"Content-Type": "application/json"})
    try:
        with request.urlopen(req, timeout=300) as resp:
            result = json.loads(resp.read().decode("utf-8"))
    except error.URLError as exc:
        raise RuntimeError(f"Ollama connection failed: {exc}") from exc

    if "error" in result:
        raise RuntimeError(result["error"])
    if "response" not in result:
        raise RuntimeError(f"Unexpected Ollama response: {result}")

    return result["response"].strip()


def write_progress(marker: str):
    (BASE_DIR / "translation_progress.txt").write_text(marker, encoding="utf-8")


def normalize_translation(text: str) -> str:
    text = text.replace("\r\n", "\n")
    text = text.replace("\r", "\n")
    text = text.strip()
    if not text:
        return text
    return text


def process_chunks():
    OUT_DIR.mkdir(exist_ok=True)
    chunk_files = sorted(CHUNK_DIR.glob("*.txt"))
    if not chunk_files:
        raise FileNotFoundError(f"No source chunks found in {CHUNK_DIR}")

    processed = 0
    failed = 0

    for src_path in chunk_files:
        out_path = OUT_DIR / src_path.name
        if out_path.exists() and out_path.stat().st_size > 0:
            processed += 1
            continue

        try:
            text = src_path.read_text(encoding="utf-8", errors="replace")
            translated = normalize_translation(translate_chunk(text))
            out_path.write_text(translated + "\n", encoding="utf-8")
            processed += 1
            write_progress(f"OK {src_path.name}")
            print(f"OK {src_path.name}")
        except Exception as exc:
            failed += 1
            err_path = OUT_DIR / f"{src_path.stem}.error.txt"
            err_path.write_text(
                f"ERROR: {exc}\n\n--- CHUNK PREVIEW ---\n{src_path.read_text(encoding='utf-8', errors='replace')[:3000]}",
                encoding="utf-8",
            )
            write_progress(f"FAILED {src_path.name}: {exc}")
            print(f"FAILED {src_path.name}: {exc}")

    print(f"Processed: {processed}, failed: {failed}")
    return processed, failed


def merge_translated_chunks():
    output_files = sorted(OUT_DIR.glob("*.txt"))
    if not output_files:
        raise FileNotFoundError(f"No translated chunk files found in {OUT_DIR}")

    merged_parts: list[str] = []
    for chunk_path in output_files:
        content = chunk_path.read_text(encoding="utf-8", errors="replace").strip()
        if content:
            merged_parts.append(content)

    if not merged_parts:
        raise RuntimeError(f"No non-empty content found in translated chunks: {OUT_DIR}")

    FINAL_OUT.write_text("\n\n".join(merged_parts).rstrip() + "\n", encoding="utf-8")
    print(f"Merged {len(merged_parts)} translated chunks into {FINAL_OUT}")


def main():
    try:
        write_source_chunks(force=False)
        process_chunks()
        merge_translated_chunks()
        print("All done.")
    except Exception as exc:
        print(f"FATAL ERROR: {exc}", file=sys.stderr)
        raise


if __name__ == "__main__":
    main()
