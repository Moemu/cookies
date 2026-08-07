# Third-party and licensing notes

## Repository license

Unless a file states otherwise, the cookies source code and documentation are distributed under the repository's [MIT License](../LICENSE). The MIT license permits use, modification, and redistribution provided that its copyright and permission notice are retained. It does not provide warranties, trademarks, service access, data rights, or rights to third-party materials.

## Included dependency: ORAG

`third_party/orag` is a Git submodule pinned to a specific upstream revision. It is independently versioned and licensed under the [MIT License](../third_party/orag/LICENSE). Keep its license, copyright notice, and any upstream notices when redistributing it. Changes to ORAG itself should normally be contributed to its upstream repository; changes here should be limited to the integration or the pinned revision.

## Included/adapted dependency: OpenCut Classic timeline

- Project: [OpenCut Classic](https://github.com/OpenCut-app/opencut-classic)
- Fixed source: `cf5e79e919144200294fb9fed22a222592a0aeea`
- License: MIT
- Copyright: 2025-2026 OpenCut
- Local notice: [`third_party/opencut-timeline/NOTICE.md`](../third_party/opencut-timeline/NOTICE.md)

The video-editing timeline command/history semantics are adapted from the fixed OpenCut Classic source snapshot. cookies does not embed the OpenCut application, project storage, database, or browser renderer.

## Concept images and project materials

The concept images under `docs/assets/` were created for this project and are included as documentation assets. They describe a direction only, not a production product or a promise of functionality. If an asset is replaced with third-party material, the contributor must document its source, license, attribution requirements, and redistribution rights in the same pull request.

## External services and models

cookies is designed to integrate with model providers, advertising platforms, analytics/data sources, storage/CDN services, and possibly fonts, stock media, music, voices, or other creative material. Each is governed by its own terms, privacy commitments, rate limits, content policies, and intellectual-property rules. Their software, API credentials, service access, output rights, and trademarks are not included under the cookies MIT license.

Before operating or distributing a deployment, operators are responsible for:

1. Obtaining valid contracts, accounts, permissions, and API credentials.
2. Reviewing the applicable provider, platform, regional privacy, and advertising rules.
3. Recording the source, license, allowed use, and expiry of customer and creative assets.
4. Ensuring that generated or transformed material is reviewed for required rights, disclosures, and platform compliance.
5. Retaining upstream license and notice files for every bundled dependency.

## Contribution requirement

By submitting a contribution, you represent that you have the right to submit it under the repository license. Do not submit customer data, secrets, personally identifiable information, unlicensed media, or material that you are not permitted to redistribute.

This file is a project notice, not legal advice. Seek qualified legal advice for a specific deployment, jurisdiction, campaign, or asset-rights question.
