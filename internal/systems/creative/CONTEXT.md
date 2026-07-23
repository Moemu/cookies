# Creative bounded context

## Ubiquitous language

- **CreativeIntake**: Creative owns the normalized, user-confirmed source for a
  planned creative output. It can be incomplete, but it is never a Strategy
  object.
- **CreativeTask**: A production unit created from a ready Intake. It owns the
  selected channel, production state, content drafts and production lineage.
- **ImageTextDraft**: A versioned, reviewable content package for one image and
  text task. It contains the post copy and the planned image sequence; it is
  not a media asset.
- **ProductionJob**: A reference from a CreativeTask to a Provider job. It
  records production lineage and does not become the task's business state.
- **CreativeDirection**: The user-selected expression of a message, including
  concept, tone and visual keywords. It can refine an upstream recommendation
  without changing that upstream object.
- **Ready Intake**: An Intake with a channel, objective, audience and core
  message. Only a ready Intake may create a CreativeTask.
