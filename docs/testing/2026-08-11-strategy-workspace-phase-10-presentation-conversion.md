# Strategy workspace phase 10: PPTX visual-fallback conversion

## Delivered boundary

- PPTX remains text-first. Low-quality Tika output is still usable before and after any conversion failure.
- Manual visual fallback now inspects two fixed capabilities: the configured LAS PDF route and the configured Gotenberg/LibreOffice converter.
- Conversion runs inside the existing background visual-fallback job. The document phase changes to `visual_conversion`, then returns to `visual_fallback`; the user may leave the drawer throughout.
- The original PPTX is never overwritten. A PDF derivative is stored under a deterministic, attempt-scoped key in the same TOS bucket.
- `platform_knowledge_document_vision_input_conversions` records source object identity, source SHA-256, converter code/version, derived object identity, derived SHA-256, size, state, and bounded errors.
- A crash while conversion is active may repeat the local conversion, but the immutable derived key and post-write readback make the object write idempotent and verifiable. Paid LAS submission still uses the stricter external-task checkpoint and reconciliation boundary.

## Failure semantics

- Gotenberg `400` and invalid/non-PDF/oversized output are terminal for that visual attempt.
- HTTP `408`, `429`, `5xx`, transport failures, and storage unavailability remain retryable background states.
- LibreOffice conversion has its own three-attempt ceiling; it does not inherit the LAS polling job's large attempt budget.
- A terminal conversion failure restores the document to `partial` when text chunks exist. It does not delete text chunks or the original upload.
- A source or derived object outside the configured shared bucket is rejected before LAS submission.

## Verification

- Unit coverage validates multipart shape, fixed filename, bounded output, PDF structure, status classification, URL policy, deterministic keys, and cross-bucket/source-lineage rejection.
- A local `gotenberg/gotenberg:8.34.0` container passed its `/health` probe and converted a generated one-slide PPTX through the real Go adapter into a validated PDF. The fixture stayed local and was removed after the smoke test.
- The real-MySQL integration path creates a PPTX, preserves its text baseline, creates and verifies the conversion checkpoint, submits the derived PDF through the visual parser seam, finalizes hybrid chunks, and confirms that the original PPTX bytes remain readable.
- Production account smoke remains deliberately pending: no Gotenberg production capacity, customer font pack, or paid LAS PPTX-derived request has been exercised by this repository change.
