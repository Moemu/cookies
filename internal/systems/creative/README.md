# Creative system

Owner: Creative team. It may reference Project, AssetVersionRef, Provider capabilities,
and confirmed Insight versions; it owns all creative business state.

## M1: independent image-and-text vertical slice

The first delivered slice is **manual Intake → Xiaohongshu image-and-text
CreativeTask → reviewable ImageTextDraft → optional Provider cover-image job**.

- `CreativeIntake` is normalized Creative-owned input. Missing objective,
  audience, or core message produces `needs_clarification`; it cannot create a
  task.
- `CreativeTask` is created only from a ready Intake and keeps its own state,
  channel, direction, drafts, and production lineage.
- Cover generation supplies `source_system=creative` and a stable CreativeTask
  ID to the Provider. Provider Jobs are not CreativeTask state.
- The M1 manual path does not read Strategy tables. The future Strategy adapter
  must create the same Intake shape through the frozen reader contract.

See [CONTEXT.md](./CONTEXT.md) for the domain vocabulary and
`api/openapi/creative-v1.yaml` for the public surface.
