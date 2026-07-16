# Generates the binary extraction fixtures in internal/ingest/testdata/.
# Stdlib only, deterministic output — rerun any time, commit the results.
# Run: python3 lab/make_fixtures.py
import zipfile
from pathlib import Path

testdata = Path(__file__).parent.parent / "internal/ingest/testdata"
testdata.mkdir(parents=True, exist_ok=True)

# --- contract.docx: headings + body paragraphs + a tab and line break ----

CONTENT_TYPES = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>"""

RELS = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>"""


def para(text, style=None):
    ppr = f'<w:pPr><w:pStyle w:val="{style}"/></w:pPr>' if style else ""
    return f"<w:p>{ppr}<w:r><w:t xml:space=\"preserve\">{text}</w:t></w:r></w:p>"


DOCUMENT = (
    '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
    "<w:body>"
    + para("Master Services Agreement", style="Title")
    + para("This Agreement is entered into by Acme Corp and Beta LLC.")
    + para("Termination", style="Heading1")
    + para("Either party may terminate this Agreement with sixty days written notice.")
    + "<w:p><w:r><w:t>Notice must be sent</w:t><w:tab/><w:t>by certified mail.</w:t></w:r></w:p>"
    + para("Payment Terms", style="Heading1")
    + para("Invoices are due net-45 from the date of receipt.")
    + "</w:body></w:document>"
)

docx_path = testdata / "contract.docx"
with zipfile.ZipFile(docx_path, "w", zipfile.ZIP_DEFLATED) as z:
    # Fixed date_time keeps the file byte-identical across runs.
    for name, data in [
        ("[Content_Types].xml", CONTENT_TYPES),
        ("_rels/.rels", RELS),
        ("word/document.xml", DOCUMENT),
    ]:
        info = zipfile.ZipInfo(name, date_time=(2026, 1, 1, 0, 0, 0))
        z.writestr(info, data)
print(f"wrote {docx_path}")

# --- contract.pdf: three pages (text, text, empty) ------------------------


def pdf_stream(lines):
    parts = ["BT /F1 12 Tf 72 720 Td 16 TL"]
    for i, line in enumerate(lines):
        line = line.replace("\\", r"\\").replace("(", r"\(").replace(")", r"\)")
        if i:
            parts.append("T*")
        parts.append(f"({line}) Tj")
    parts.append("ET")
    return " ".join(parts).encode()


PAGES = [
    pdf_stream([
        "MASTER SERVICES AGREEMENT",
        "This Agreement may be terminated by either party",
        "with sixty days written notice.",
    ]),
    pdf_stream([
        "PAYMENT TERMS",
        "Invoices are due net-45 from the date of receipt.",
        "Case No. 2024-CV-01847 governs disputes.",
    ]),
    b"",  # deliberately empty page: extraction must not choke or mis-number
]

objects = {}
n_pages = len(PAGES)
first_page = 3
font_obj = first_page + 2 * n_pages

kids = " ".join(f"{first_page + 2 * i} 0 R" for i in range(n_pages))
objects[1] = b"<< /Type /Catalog /Pages 2 0 R >>"
objects[2] = f"<< /Type /Pages /Kids [{kids}] /Count {n_pages} >>".encode()
for i, stream in enumerate(PAGES):
    page_num = first_page + 2 * i
    objects[page_num] = (
        f"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] "
        f"/Resources << /Font << /F1 {font_obj} 0 R >> >> "
        f"/Contents {page_num + 1} 0 R >>"
    ).encode()
    objects[page_num + 1] = (
        f"<< /Length {len(stream)} >>\nstream\n".encode() + stream + b"\nendstream"
    )
objects[font_obj] = b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"

out = bytearray(b"%PDF-1.4\n")
offsets = {}
for num in sorted(objects):
    offsets[num] = len(out)
    out += f"{num} 0 obj\n".encode() + objects[num] + b"\nendobj\n"

xref_at = len(out)
count = len(objects) + 1
out += f"xref\n0 {count}\n".encode()
out += b"0000000000 65535 f \n"
for num in sorted(objects):
    out += f"{offsets[num]:010d} 00000 n \n".encode()
out += (
    f"trailer\n<< /Size {count} /Root 1 0 R >>\nstartxref\n{xref_at}\n%%EOF\n"
).encode()

pdf_path = testdata / "contract.pdf"
pdf_path.write_bytes(out)
print(f"wrote {pdf_path}")
