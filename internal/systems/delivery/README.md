# Delivery system

Owner: Delivery team. It owns plans, change sets, platform entities, and
evidence; shared Computer Use only provides a controlled execution capability.

The Phase C authority spine is:

`DeliveryIntent + PlatformConfiguration + immutable facts -> DeliveryDecision -> DecisionSelection -> CompiledDeliveryWorkflow -> ready_for_final_approval`

Decision generation and workflow compilation are deterministic and side-effect free. Persisting a selection creates a new immutable platform-configuration version, but never mutates the Plan, creates a formal approval, or enables a platform write. Every compiled `remote_write` step is blocked with `PHASE_C_REMOTE_WRITE_PROHIBITED`; the database also rejects workflows whose `remote_write_enabled` is true.

The Recommendation lifecycle is not an active optimization model. New generate/accept/reject operations are restricted to owner-scoped historical Tour runs; non-Tour projects use DeliveryDecision exclusively.
