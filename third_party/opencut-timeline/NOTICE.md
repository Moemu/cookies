# OpenCut Timeline Notice

The cookies video-editing timeline command model is adapted from OpenCut Classic.

- Upstream: https://github.com/OpenCut-app/opencut-classic
- Fixed commit: `cf5e79e919144200294fb9fed22a222592a0aeea`
- License: MIT, reproduced in `LICENSE`
- Copyright: 2025-2026 OpenCut

Reviewed upstream sources:

- `apps/web/src/commands/timeline/element/move-elements.ts`
- `apps/web/src/commands/timeline/element/split-elements.ts`
- `apps/web/src/core/managers/commands.ts`
- `apps/web/src/timeline/controllers/resize-controller.ts`
- `apps/web/src/timeline/controllers/zoom-controller.ts`
- `apps/web/src/timeline/snapping/`

The adapted implementation is maintained in
`src/features/video-editing/timeline.ts`. It replaces OpenCut's EditorCore,
WASM media time, project storage, and rendering dependencies with cookies'
own immutable timeline, AssetVersionRef, Go API, and FFmpeg render pipeline.
