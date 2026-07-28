# Delivery domain context

Delivery owns the controlled transition from an immutable CreativePackage to an auditable advertising-platform change.

- **DeliveryPlan**: the approved intent, budget window, and immutable CreativePackage reference.
- **ChangeSet**: one versioned proposal to change a platform. It is the only object that can be preflighted, approved, executed, or rolled back.
- **Execution**: one attempt to apply an approved ChangeSet.
- **Evidence**: the durable proof of what an Execution did. MVP evidence explicitly records local simulation and must never imply a real platform write.

Delivery does not own Creative content, Provider jobs, project identity, post-launch analysis, or reusable experience rules.
