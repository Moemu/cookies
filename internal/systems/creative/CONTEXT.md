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
- **Ready Intake**: An Intake with a channel, objective, audience and core
  message. Only a ready Intake may create a CreativeTask.
