# Database migration rules

Each owner keeps forward-compatible SQL migrations in its own directory. A
module may only migrate its own schema. New tenant-scoped tables include
`organization_id`; indexes and unique constraints use it as a leading key when
appropriate.

Use expand → migrate → contract changes. Do not ship destructive migrations or
run uncontrolled migrations during application startup.
