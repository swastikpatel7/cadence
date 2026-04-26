-- Extensions for Cadence's database.
-- pgvector is reserved for v2 RAG; installed now so migrations stay simple.
-- pgcrypto provides gen_random_uuid() and HMAC functions.

CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
