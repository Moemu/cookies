# Platform event catalog

| Event | Owner | Initial consumers | Schema |
| --- | --- | --- | --- |
| `model.job.completed.v1` | Provider platform | Assets, Creative, Insights, Workflow | `model-job-completed-v1.schema.json` |
| `asset.ready.v1` | Assets platform | Creative, Insights, Delivery, Knowledge | `asset-ready-v1.schema.json` |

The envelope and every payload schema are immutable after publication. A
breaking change requires a new versioned event name and schema file.
