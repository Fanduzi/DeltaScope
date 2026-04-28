CREATE INDEX idx_docs_body ON docs USING gin (body);
