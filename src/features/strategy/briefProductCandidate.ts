import type { BriefDocument, BriefProductCandidate } from './types'

export type BriefFieldOperation = { fieldPath: string; value: unknown }

function normalizedFact(value: string) {
  return value.trim().replace(/\s+/g, ' ')
}

function uniqueFacts(values: string[] | undefined) {
  const seen = new Set<string>()
  const result: string[] = []
  for (const value of values ?? []) {
    const normalized = normalizedFact(value)
    if (!normalized || seen.has(normalized)) continue
    seen.add(normalized)
    result.push(value.trim())
  }
  return result
}

function creativeFactsForSelection(
  current: string[] | undefined,
  previous: string[] | undefined,
  selected: string[] | undefined,
) {
  const previousFacts = new Set(uniqueFacts(previous).map(normalizedFact))
  const sharedFacts = uniqueFacts(current).filter(value => !previousFacts.has(normalizedFact(value)))
  return uniqueFacts([...sharedFacts, ...uniqueFacts(selected)])
}

/**
 * Projects one user-selected product candidate into the fixed Brief fields in
 * one PATCH request. Empty candidate fields are deliberately written too, so
 * switching products cannot retain facts from the previous product.
 */
export function briefProductCandidateOperations(
  document: BriefDocument,
  candidate: BriefProductCandidate,
): BriefFieldOperation[] {
  const candidates = document.product?.candidates ?? []
  const previous = candidates.find(value => normalizedFact(value.name) === normalizedFact(document.product?.name ?? ''))

  return [
    { fieldPath: 'product.name', value: candidate.name.trim() },
    { fieldPath: 'product.category', value: candidate.category?.trim() ?? '' },
    { fieldPath: 'product.selling_points', value: uniqueFacts(candidate.selling_points) },
    { fieldPath: 'product.evidence', value: uniqueFacts(candidate.evidence) },
    {
      fieldPath: 'creative.mandatory_elements',
      value: creativeFactsForSelection(
        document.creative?.mandatory_elements,
        previous?.mandatory_elements,
        candidate.mandatory_elements,
      ),
    },
    {
      fieldPath: 'creative.prohibited_claims',
      value: creativeFactsForSelection(
        document.creative?.prohibited_claims,
        previous?.prohibited_claims,
        candidate.prohibited_claims,
      ),
    },
  ]
}
