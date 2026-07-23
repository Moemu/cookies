// Package projectcontext contains Project-module behavior around the shared
// project contract. The aliases preserve the existing package seam while the
// frozen cross-line shapes live in contract.
package projectcontext

import "github.com/shikanon/cookies/internal/platform/contract"

type BrandID = contract.BrandID
type ProductID = contract.ProductID
type Reference = contract.ProjectContext
