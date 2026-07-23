# Creative migrations

Owner: Creative team.

`20260723150000_creative_image_text_m1.up.sql` creates Creative-owned Intakes,
Tasks, image-and-text drafts, and production-lineage records. It deliberately
does not add foreign keys to Strategy or Provider tables; those are cross-module
references resolved through their APIs.
