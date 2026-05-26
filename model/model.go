package model

import internal "github.com/init0-lux/vcf-toolkit/internal/model"

// Public SDK surface: these are type aliases over the internal implementation.
// This keeps the CLI free to use internal packages while making the module
// usable as a library.

type SourceInfo = internal.SourceInfo
type Contact = internal.Contact

type NormalizedName = internal.NormalizedName
type NormalizedPhone = internal.NormalizedPhone
type NormalizedEmail = internal.NormalizedEmail

type MergedContact = internal.MergedContact
type DedupeReport = internal.DedupeReport
type DedupeResult = internal.DedupeResult
type ParseError = internal.ParseError
