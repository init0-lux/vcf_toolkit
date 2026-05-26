package normalize

import internal "github.com/init0-lux/vcf-toolkit/internal/normalize"

// Public SDK wrapper around the internal implementation.
// Consumers should import this package, not the internal one.

type NameConfig = internal.NameConfig
type PhoneConfig = internal.PhoneConfig
type EmailConfig = internal.EmailConfig
type NameLLM = internal.NameLLM
type HTTPNameLLM = internal.HTTPNameLLM

var ErrNoLLMClient = internal.ErrNoLLMClient

var NormalizeName = internal.NormalizeName
var NormalizePhone = internal.NormalizePhone
var NormalizeEmail = internal.NormalizeEmail

// Helpers used by other packages (dedupe) and by SDK consumers.
var NormalizeEmailStr = internal.NormalizeEmailStr
var PhoneDigits = internal.PhoneDigits
