-- Irreversible safety cleanup: LAS crop URLs are temporary provider output and
-- must not remain in durable project knowledge. The source document preview is
-- the stable visual reference after this migration.
UPDATE platform_knowledge_chunks
SET
  text_sha256 = SHA2(REGEXP_REPLACE(
    text,
    '!\\[[^]]*\\]\\(https?://[^)]+\\)',
    '[文档图片已省略，请在原文预览中查看]'
  ), 256),
  text = REGEXP_REPLACE(
    text,
    '!\\[[^]]*\\]\\(https?://[^)]+\\)',
    '[文档图片已省略，请在原文预览中查看]'
  )
WHERE parser_code = 'document_vision'
  AND text REGEXP '!\\[[^]]*\\]\\(https?://[^)]+\\)';

-- Fail closed for any legacy temporary resource that was not represented as a
-- Markdown image. Re-parsing can recover the extracted text from the source.
UPDATE platform_knowledge_chunks
SET
  text_sha256 = SHA2('[视觉解析结果包含已过期临时资源，请重新解析并在原文预览中查看]', 256),
  text = '[视觉解析结果包含已过期临时资源，请重新解析并在原文预览中查看]'
WHERE parser_code = 'document_vision'
  AND (text LIKE '%X-Tos-%' OR text LIKE '%/las-serving-tmp/%');
