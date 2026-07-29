# Creative bounded context

## Ubiquitous language

- **CreativeIntake**: Creative owns the normalized, user-confirmed source for a
  planned creative output. It can be incomplete, but it is never a Strategy
  object.
- **CreativeTask**: A production unit created from a ready Intake. It owns the
  selected channel, production state, content drafts and production lineage.
- **ImageTextDraft**: The editable working revision for one image-and-text
  task. It contains the post copy and planned image sequence; it is not a
  media asset or a cross-system hand-off.
- **CreativeVersion**: An immutable Creative-owned snapshot frozen from one
  Draft revision. Checks, review, approval, delivery and later systems refer
  to this stable identity rather than to an editable Draft or a Provider job.
- **CreativeCheck**: The recorded result of evaluating a frozen CreativeVersion
  against the agreed image group, mandatory elements and prohibited claims.
  A failed check is evidence, not an edit to the frozen version.
- **CreativePackage**: The delivery-safe Creative hand-off built only from an
  approved CreativeVersion. It contains frozen copy and `AssetVersionRef`s;
  Delivery and Insights never receive a mutable CreativeTask.
- **ProductionJob**: A reference from a CreativeTask to a Provider job. It
  records production lineage and does not become the task's business state.
- **CreativeDirection**: The user-selected expression of a message, including
  concept, tone and visual keywords. It can refine an upstream recommendation
  without changing that upstream object.
- **CreativeSourceVersion**: The immutable upstream version deliberately
  selected for one creative effort. It may be a confirmed Brief, an approved
  StrategyPackage or a development fixture; later upstream versions never
  alter an existing CreativeIntake.
- **VideoTemplateRecipe**: Creative's versioned production grammar for a video
  pattern. It defines required facts and assets, motion phases and preservation
  rules, but it is not the final model prompt.
- **PromptPackage**: An immutable, traceable compilation of one CreativeIntake
  and one VideoTemplateRecipe into structured directions and the exact model
  prompt shown for approval.
- **GenerationSpec**: The frozen, approved combination of a PromptPackage,
  conditioning assets and media settings used to request model production.
- **Candidate**: One model-produced output under evaluation. Provider success
  makes a Candidate available; it does not make the Candidate approved.
- **Ready Intake**: An Intake with a channel, objective, audience and core
  message. Only a ready Intake may create a CreativeTask.
