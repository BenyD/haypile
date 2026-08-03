#!/usr/bin/env sh
# Downloads the real-world eval corpus: ~50 public PDFs with stable URLs.
# The corpus stays out of git (like the model weights); this script is the
# reproducible way to get it. Re-running skips files already present.
#
#   ./eval/realworld/fetch-corpus.sh
#
# Sources are chosen for layout diversity, not topic: two-column academic
# papers (arxiv), long structured law (EUR-Lex), table-heavy publications
# (NIST, IRS), and plain single-column prose (RFCs, shareholder letters).
set -eu

dir="$(dirname "$0")/corpus"
mkdir -p "$dir/papers" "$dir/rfcs" "$dir/law" "$dir/gov" "$dir/finance"

# Empty files are failed downloads from an earlier run; clear them so
# they get retried rather than silently shipping a smaller corpus.
find "$dir" -name '*.pdf' -empty -delete 2>/dev/null || true

fetch() {
  dest="$dir/$1"
  url="$2"
  if [ -s "$dest" ]; then
    echo "have  $1"
    return
  fi
  echo "fetch $1"
  curl -fsSL --retry 3 --max-time 120 -o "$dest" "$url" || {
    echo "MISS  $1 ($url)" >&2
    rm -f "$dest"
    return
  }
  # Some hosts answer bots with empty 200s or HTML challenge pages; a file
  # that does not start with %PDF is a miss, not a document.
  if [ "$(head -c 4 "$dest")" != "%PDF" ]; then
    echo "MISS  $1 (not a PDF: $url)" >&2
    rm -f "$dest"
  fi
}

# Academic papers (two-column and single-column, dense math, figures)
fetch papers/attention-is-all-you-need.pdf   "https://arxiv.org/pdf/1706.03762"
fetch papers/bert.pdf                        "https://arxiv.org/pdf/1810.04805"
fetch papers/gpt3.pdf                        "https://arxiv.org/pdf/2005.14165"
fetch papers/sentence-bert.pdf               "https://arxiv.org/pdf/1908.10084"
fetch papers/dense-passage-retrieval.pdf     "https://arxiv.org/pdf/2004.04906"
fetch papers/beir-benchmark.pdf              "https://arxiv.org/pdf/2104.08663"
fetch papers/hnsw.pdf                        "https://arxiv.org/pdf/1603.09320"
fetch papers/e5-embeddings.pdf               "https://arxiv.org/pdf/2212.03533"
fetch papers/mteb-benchmark.pdf              "https://arxiv.org/pdf/2210.07316"
fetch papers/bge-c-pack.pdf                  "https://arxiv.org/pdf/2309.07597"
fetch papers/rag-survey.pdf                  "https://arxiv.org/pdf/2312.10997"
fetch papers/rag-lewis.pdf                   "https://arxiv.org/pdf/2005.11401"
fetch papers/contriever.pdf                  "https://arxiv.org/pdf/2112.09118"
fetch papers/bert-reranking.pdf              "https://arxiv.org/pdf/1901.04085"
fetch papers/word2vec.pdf                    "https://arxiv.org/pdf/1301.3781"
fetch papers/fasttext-subword.pdf            "https://arxiv.org/pdf/1607.04606"
fetch papers/llama2.pdf                      "https://arxiv.org/pdf/2307.09288"
fetch papers/llama.pdf                       "https://arxiv.org/pdf/2302.13971"
fetch papers/instructgpt.pdf                 "https://arxiv.org/pdf/2203.02155"
fetch papers/moco.pdf                        "https://arxiv.org/pdf/1911.05722"

# RFCs (single-column prose, section-heavy; v3-era RFCs all ship PDFs)
fetch rfcs/rfc9110-http-semantics.pdf  "https://www.rfc-editor.org/rfc/rfc9110.pdf"
fetch rfcs/rfc9111-http-caching.pdf    "https://www.rfc-editor.org/rfc/rfc9111.pdf"
fetch rfcs/rfc9112-http11.pdf          "https://www.rfc-editor.org/rfc/rfc9112.pdf"
fetch rfcs/rfc9113-http2.pdf           "https://www.rfc-editor.org/rfc/rfc9113.pdf"
fetch rfcs/rfc9114-http3.pdf           "https://www.rfc-editor.org/rfc/rfc9114.pdf"
fetch rfcs/rfc9000-quic.pdf            "https://www.rfc-editor.org/rfc/rfc9000.pdf"
fetch rfcs/rfc9001-quic-tls.pdf        "https://www.rfc-editor.org/rfc/rfc9001.pdf"
fetch rfcs/rfc9002-quic-recovery.pdf   "https://www.rfc-editor.org/rfc/rfc9002.pdf"
fetch rfcs/rfc9293-tcp.pdf             "https://www.rfc-editor.org/rfc/rfc9293.pdf"
fetch rfcs/rfc8949-cbor.pdf            "https://www.rfc-editor.org/rfc/rfc8949.pdf"
fetch rfcs/rfc9106-argon2.pdf          "https://www.rfc-editor.org/rfc/rfc9106.pdf"

# US federal law (long, deeply structured, legal language; the lawyer
# case). govinfo.gov explicitly supports programmatic access. EUR-Lex
# does not: it sits behind a WAF challenge and returns empty 200s to
# anything that is not a browser.
fetch law/hipaa.pdf              "https://www.govinfo.gov/content/pkg/PLAW-104publ191/pdf/PLAW-104publ191.pdf"
fetch law/sarbanes-oxley.pdf     "https://www.govinfo.gov/content/pkg/PLAW-107publ204/pdf/PLAW-107publ204.pdf"
fetch law/gramm-leach-bliley.pdf "https://www.govinfo.gov/content/pkg/PLAW-106publ102/pdf/PLAW-106publ102.pdf"
fetch law/dodd-frank.pdf         "https://www.govinfo.gov/content/pkg/PLAW-111publ203/pdf/PLAW-111publ203.pdf"

# NIST publications (table-heavy, control catalogs, dense structure)
fetch gov/nist-800-63b-authentication.pdf "https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-63b.pdf"
fetch gov/nist-800-53r5-controls.pdf      "https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-53r5.pdf"
fetch gov/nist-800-207-zero-trust.pdf     "https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-207.pdf"
fetch gov/nist-800-171r2.pdf              "https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-171r2.pdf"
fetch gov/fips-203-mlkem.pdf              "https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.203.pdf"
fetch gov/fips-204-mldsa.pdf              "https://nvlpubs.nist.gov/nistpubs/FIPS/NIST.FIPS.204.pdf"

# IRS (forms and instructions: the hardest tables in common circulation)
fetch gov/irs-1040-instructions.pdf "https://www.irs.gov/pub/irs-pdf/i1040gi.pdf"
fetch gov/irs-pub15-employer.pdf    "https://www.irs.gov/pub/irs-pdf/p15.pdf"
fetch gov/irs-pub17.pdf             "https://www.irs.gov/pub/irs-pdf/p17.pdf"

# Shareholder letters (plain prose, financial tables)
fetch finance/berkshire-2023.pdf "https://www.berkshirehathaway.com/letters/2023ltr.pdf"
fetch finance/berkshire-2022.pdf "https://www.berkshirehathaway.com/letters/2022ltr.pdf"
fetch finance/berkshire-2021.pdf "https://www.berkshirehathaway.com/letters/2021ltr.pdf"

count=$(find "$dir" -name '*.pdf' | wc -l | tr -d ' ')
echo "corpus: $count PDFs in $dir"
