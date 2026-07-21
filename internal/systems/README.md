# Vertical systems

The product documentation calls these four independent vertical systems. Each
owns its routes, state machines, database schema, migrations, API, permissions,
metrics, and release cadence. Systems must not read or write another system's
tables directly.

- `strategy`: conversations, Briefs, research, and strategies.
- `creative`: creative tasks, directions, versions, and packages.
- `insights`: asset features, analysis runs, insights, and experiences.
- `delivery`: delivery plans, change sets, platform entities, and evidence.
