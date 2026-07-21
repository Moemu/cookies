# Assets platform module

Owner: Project and Assets team.

This module owns project media assets, versions, source provenance, rights,
uploads, generated-result intake, and durable `ProjectAssetRef` creation. It does not
call model-vendor SDKs; it receives verified provider generation results through
an explicit intake API.
