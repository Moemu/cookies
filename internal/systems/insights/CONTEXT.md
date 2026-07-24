# Insights domain context

Insights owns evidence-backed interpretation and reusable learning.

- **InsightReport**: a reviewable interpretation of one Delivery Execution and its Evidence.
- **Experience**: a confirmed, reusable conclusion with applicability conditions and counterexamples.
- **PreLaunchInsight**: a read model that returns confirmed Experience references before the next campaign decision.
- **PerformanceOverview**: a read-only projection of Delivery evidence. It does not become a second source of truth for execution state.

Insights does not own media Assets, Creative versions, Delivery plans, platform executions, or raw provider jobs.
