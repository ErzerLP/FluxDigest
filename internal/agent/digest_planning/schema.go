package digest_planning

import domaindigest "rss-platform/internal/domain/digest"

// CandidateArticle 复用共享 digest DTO，避免 agent 包拥有跨层共享结构。
type CandidateArticle = domaindigest.CandidateArticle

// SectionItem 复用共享 digest DTO，保留稳定文章引用。
type SectionItem = domaindigest.SectionItem

// Section 复用共享 digest DTO。
type Section = domaindigest.Section

// Plan 复用共享 digest DTO。
type Plan = domaindigest.Plan
