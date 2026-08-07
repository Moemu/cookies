# Upstream policy

OpenCut Classic commit `cf5e79e919144200294fb9fed22a222592a0aeea` is the only approved source snapshot for the first editor phase. It is archived upstream and is not a runtime or package dependency.

Any additional copied or adapted source must update `NOTICE.md`, pass timeline unit and end-to-end tests, and receive a license/dependency review. The floating OpenCut `main` branch must not be merged automatically.

The removal boundary is `src/features/video-editing/timeline.ts`: cookies API and database records always use `editing-timeline/v1` or a future cookies-owned schema, so removing the adapted interaction code requires no persisted-data migration.
