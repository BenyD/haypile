# Generates reference goldens for the pure-Go bundled embedder tests:
# token ids from the HF tokenizer and embeddings from sentence-transformers.
# Run: lab/.venv/bin/python lab/make_goldens.py
import json
from pathlib import Path

from sentence_transformers import SentenceTransformer

CASES = [
    "The quick brown fox jumps over the lazy dog.",
    "agreement cancellation",
    "The contract may be terminated by either party with 30 days notice.",
    "Café résumé naïve — accents must be stripped.",
    "it's a test-case, isn't it? (yes!)",
    "Case No. 2024-CV-01847 filed in the district court",
    "深度学习模型可以处理中文文本",
    "MiXeD CaSe with EMOJI 😀 and    extra   spaces",
    "word " * 400,  # forces truncation at max_seq_length
]

model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")
tok = model.tokenizer

cases = []
for text in CASES:
    ids = tok(text, truncation=True, max_length=256)["input_ids"]
    emb = model.encode(text, normalize_embeddings=True)
    cases.append({
        "text": text,
        "ids": ids,
        "embedding": [round(float(x), 6) for x in emb],
    })

out = Path(__file__).parent.parent / "internal/embed/bundled/testdata/goldens.json"
out.write_text(json.dumps({"cases": cases}, ensure_ascii=False, indent=1))
print(f"wrote {out} ({len(cases)} cases)")
