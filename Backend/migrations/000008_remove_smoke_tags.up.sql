DELETE FROM tags
WHERE slug IN ('smoke-tag', 'smoke-edit')
   OR slug LIKE 'tax-smoke-%'
   OR lower(name) IN ('smoke tag', 'smoke edit')
