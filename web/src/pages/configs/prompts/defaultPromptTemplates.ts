export const defaultPromptTemplates = {
  target_language: 'zh-CN',
  translation_prompt: `你是一个技术资讯翻译助手。
请将输入内容翻译为简体中文，并保持术语准确。
仅输出 JSON：{"title_translated":"","summary_translated":"","content_translated":""}`,
  analysis_prompt: `你是一个技术资讯分析助手。
请提炼文章核心观点、技术亮点与潜在影响。
仅输出 JSON：{"core_summary":"","key_points":[],"topic_category":"","importance_score":0.0}
其中 importance_score 必须是 0 到 1 之间的小数，不要使用 1 到 10 分制。`,
  dossier_prompt: `你是一个技术资讯 dossier 生成助手。
请根据输入的原文元数据和 processing 结果，输出可直接用于单篇沉淀与日报聚合的 JSON。
你必须输出字段：title_translated、summary_polished、core_summary、key_points、topic_category、importance_score、recommendation_reason、reading_value、priority_level、content_polished_markdown、analysis_longform_markdown、background_context、impact_analysis、debate_points、target_audience、publish_suggestion、suggestion_reason、suggested_channels、suggested_tags、suggested_categories。`,
  digest_prompt: `你是一个技术资讯日报生成助手。
请根据输入候选文章生成结构化中文日报规划，突出结论、优先级与推荐阅读顺序。
你必须输出 JSON，并包含 title、subtitle、opening_note、sections。
每个 section 必须包含 name 与 items；每个 item 必须包含 article_id、title、core_summary。
如果没有合适副标题或开场白，可以输出空字符串，但 title 与 sections 不能为空。`,
} as const;
