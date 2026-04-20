import { Alert, Button, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';

import { PageHeader } from '../../../components/common/PageHeader';
import { ChoiceCardGroup } from '../../../components/forms/ChoiceCardGroup';
import { SecretField } from '../../../components/forms/SecretField';
import { StatusBadge } from '../../../components/status/StatusBadge';
import {
  useRunDailyDigest,
  useSavePublishConfig,
  useSaveSchedulerConfig,
  useTestPublishConfig,
} from '../../../services/mutations/admin';
import { useAdminConfigs, useAdminStatus } from '../../../services/queries/admin';
import type {
  ArticlePublishMode,
  ArticleReviewMode,
  SecretInput,
  UpdatePublishConfigInput,
  UpdateSchedulerConfigInput,
} from '../../../types/admin';

interface PublishConfigFormValues {
  provider: 'halo' | 'markdown_export';
  halo_base_url: string;
  output_dir: string;
  article_publish_mode: ArticlePublishMode;
  article_review_mode: ArticleReviewMode;
  digest_schedule_time: string;
  digest_enabled: boolean;
}

const providerOptions = [
  {
    value: 'halo',
    label: 'Halo 发布',
    description: '直接推送日报到 Halo，并为后续单篇文章发布保留统一入口。',
  },
  {
    value: 'markdown_export',
    label: 'Markdown 导出',
    description: '把日报或单篇文章导出为 Markdown，适合静态站或外部脚本接力。',
  },
] as const;

const articlePublishModeOptions = [
  {
    value: 'digest_only',
    label: '只发日报',
    description: '单篇文章只做翻译、分析与入库，不自动进入外部发布通道。',
  },
  {
    value: 'suggested',
    label: '部分发送 + 审核',
    description: '仅 AI 判断值得发布的文章进入后续流程，适合更稳妥的个人使用方式。',
  },
  {
    value: 'all',
    label: '全部发送',
    description: '所有处理完成的文章都进入发布流程，适合重度自动化场景。',
  },
] as const;

const articleReviewModeOptions = [
  {
    value: 'manual_review',
    label: '人工审核',
    description: '进入发布队列后等待人工确认，适合需要把关标题和措辞的场景。',
  },
  {
    value: 'auto_publish',
    label: '自动发布',
    description: '生成完成后直接发布，适合稳定提示词和固定输出渠道。',
  },
] as const;

function valueOrDash(value?: string) {
  return value?.trim() ? value : '—';
}

export function PublishConfigPage() {
  const configQuery = useAdminConfigs();
  const statusQuery = useAdminStatus();
  const savePublishMutation = useSavePublishConfig();
  const saveSchedulerMutation = useSaveSchedulerConfig();
  const runDailyDigestMutation = useRunDailyDigest();
  const testMutation = useTestPublishConfig();
  const [secretInput, setSecretInput] = useState<SecretInput>({ mode: 'keep' });
  const [saveGuidance, setSaveGuidance] = useState<string>();
  const [saveMessage, setSaveMessage] = useState<string>();
  const [runMessage, setRunMessage] = useState<string>();

  const currentConfig = configQuery.data?.publish;
  const schedulerConfig = configQuery.data?.scheduler;
  const configReady = configQuery.isSuccess;
  const integration = statusQuery.data?.integrations?.publisher;

  const { register, handleSubmit, reset, setValue, watch } = useForm<PublishConfigFormValues>({
    defaultValues: {
      provider: 'halo',
      halo_base_url: '',
      output_dir: '',
      article_publish_mode: 'digest_only',
      article_review_mode: 'manual_review',
      digest_schedule_time: '07:00',
      digest_enabled: true,
    },
  });

  const provider = watch('provider');
  const articlePublishMode = watch('article_publish_mode');
  const articleReviewMode = watch('article_review_mode');
  const digestEnabled = watch('digest_enabled');

  useEffect(() => {
    if (!currentConfig) {
      return;
    }

    const nextProvider = currentConfig.provider === 'markdown_export' ? 'markdown_export' : 'halo';
    reset({
      provider: nextProvider,
      halo_base_url: currentConfig.halo_base_url ?? '',
      output_dir: currentConfig.output_dir ?? '',
      article_publish_mode: currentConfig.article_publish_mode ?? 'digest_only',
      article_review_mode: currentConfig.article_review_mode ?? 'manual_review',
      digest_schedule_time: schedulerConfig?.schedule_time ?? '07:00',
      digest_enabled: schedulerConfig?.enabled ?? true,
    });
    setSecretInput({
      mode: nextProvider === 'halo' && !currentConfig.halo_token?.is_set ? 'replace' : 'keep',
      value: '',
    });
    setSaveGuidance(undefined);
    setSaveMessage(undefined);
  }, [currentConfig, schedulerConfig, reset]);

  const connectionStatus = integration?.configured
    ? integration.last_test_status ?? 'configured'
    : 'missing';
  const isSaving = savePublishMutation.isPending || saveSchedulerMutation.isPending;
  const saveError = savePublishMutation.error ?? saveSchedulerMutation.error;

  const outputPolicySummary = useMemo(() => {
    switch (articlePublishMode) {
      case 'all':
        return '所有单篇文章都会进入发布队列，可结合审核策略控制是否先人工确认。';
      case 'suggested':
        return '仅 AI 建议值得发布的文章进入后续流程，适合“部分发送 + 审核”。';
      default:
        return '只发布每日汇总日报，单篇文章仅保存为内部资产与接口数据。';
    }
  }, [articlePublishMode]);

  const providerSummary = useMemo(() => {
    switch (provider) {
      case 'markdown_export':
        return 'Markdown 导出';
      default:
        return 'Halo 发布';
    }
  }, [provider]);

  const reviewSummary = useMemo(() => {
    switch (articleReviewMode) {
      case 'auto_publish':
        return '自动发布';
      default:
        return '人工审核';
    }
  }, [articleReviewMode]);

  const onSubmit = async (values: PublishConfigFormValues) => {
    if (!configReady) {
      return;
    }

    if (values.provider === 'halo' && secretInput.mode === 'replace' && !secretInput.value?.trim()) {
      setSaveGuidance('Halo 模式下当前处于“替换密钥”模式，请先输入 Halo Token 再保存。');
      return;
    }

    const publishPayload: UpdatePublishConfigInput = {
      provider: values.provider,
      halo_base_url: values.halo_base_url,
      output_dir: values.output_dir,
      article_publish_mode: values.article_publish_mode,
      article_review_mode: values.article_review_mode,
      halo_token:
        secretInput.mode === 'replace'
          ? { mode: 'replace', value: secretInput.value ?? '' }
          : { mode: secretInput.mode },
    };
    const schedulerPayload: UpdateSchedulerConfigInput = {
      enabled: values.digest_enabled,
      schedule_time: values.digest_schedule_time,
      timezone: schedulerConfig?.timezone ?? 'Asia/Shanghai',
    };

    try {
      setSaveGuidance(undefined);
      setSaveMessage(undefined);
      await Promise.all([
        savePublishMutation.mutateAsync(publishPayload),
        saveSchedulerMutation.mutateAsync(schedulerPayload),
      ]);
      setSaveMessage('发布设置已保存。');
    } catch {
      setSaveMessage(undefined);
    }
  };

  const handleManualRun = async () => {
    try {
      const result = await runDailyDigestMutation.mutateAsync({ force: true });
      setRunMessage(`已触发手动日报：${result.digest_date ?? '今日'}（${result.status ?? 'accepted'}）`);
    } catch {
      setRunMessage(undefined);
    }
  };

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="Publish"
          subtitle="配置日报时间、单篇文章发布流程、审核策略与发布通道，并支持手动重跑日报。"
          actions={
            <div className="button-cluster">
              <Button
                onClick={() => void handleManualRun()}
                loading={runDailyDigestMutation.isPending}
                disabled={!configReady}
              >
                手动生成日报
              </Button>
              <Button
                onClick={() => testMutation.mutate()}
                loading={testMutation.isPending}
                disabled={!configReady}
              >
                测试连接
              </Button>
              <Button
                type="primary"
                onClick={() => void handleSubmit(onSubmit)()}
                loading={isSaving}
                disabled={!configReady}
              >
                保存配置
              </Button>
            </div>
          }
        />

        {configQuery.isLoading ? (
          <div className="empty-state">
            <Spin />
          </div>
        ) : null}

        {configQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message="发布配置读取失败"
            description={configQuery.error instanceof Error ? configQuery.error.message : '未知错误'}
          />
        ) : null}

        {saveMessage ? <Alert type="success" showIcon message={saveMessage} /> : null}
        {saveError ? (
          <Alert
            type="error"
            showIcon
            message="发布配置保存失败"
            description={saveError instanceof Error ? saveError.message : '未知错误'}
          />
        ) : null}
        {saveGuidance ? <Alert type="warning" showIcon message={saveGuidance} /> : null}
        {runMessage ? <Alert type="success" showIcon message={runMessage} /> : null}
        {runDailyDigestMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="手动生成日报失败"
            description={
              runDailyDigestMutation.error instanceof Error
                ? runDailyDigestMutation.error.message
                : '未知错误'
            }
          />
        ) : null}
        {testMutation.isSuccess ? (
          <Alert
            type="info"
            showIcon
            message={`连接测试：${testMutation.data.status ?? 'unknown'}`}
            description={testMutation.data.message ?? '已收到测试结果'}
          />
        ) : null}
        {testMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="发布通道测试失败"
            description={testMutation.error instanceof Error ? testMutation.error.message : '未知错误'}
          />
        ) : null}
        <Alert
          type="info"
          showIcon
          message="测试连接会校验当前已保存的发布配置；切换 Provider 或修改 Token 后请先保存。"
        />

        {configReady ? (
          <form
            className="config-form-grid"
            onSubmit={(event) => void handleSubmit(onSubmit)(event)}
          >
            <section className="data-card form-card">
              <div className="section-heading-row compact-row">
                <div>
                  <p className="section-eyebrow">Delivery channel</p>
                  <h2 className="section-title">发布通道</h2>
                  <p className="section-description">
                    先选定 FluxDigest 的最终输出目标，再补充对应的连接参数与鉴权信息。
                  </p>
                </div>
                <StatusBadge status={connectionStatus} />
              </div>

              <ChoiceCardGroup
                name="publish-provider"
                label="Provider"
                value={provider}
                options={[...providerOptions]}
                onChange={(nextProvider) => {
                  setValue('provider', nextProvider, { shouldDirty: true, shouldTouch: true });
                }}
              />
              <input type="hidden" {...register('provider')} />

              {provider === 'halo' ? (
                <>
                  <div className="form-stack">
                    <label className="form-label" htmlFor="publish-halo-base-url">
                      Halo Base URL
                    </label>
                    <input
                      id="publish-halo-base-url"
                      className="console-input"
                      {...register('halo_base_url')}
                    />
                  </div>

                  <SecretField
                    id="publish-halo-token"
                    label="Halo Token"
                    currentSecret={currentConfig?.halo_token}
                    value={secretInput}
                    onChange={setSecretInput}
                  />
                </>
              ) : (
                <div className="form-stack">
                  <label className="form-label" htmlFor="publish-output-dir">
                    Output Directory
                  </label>
                  <input
                    id="publish-output-dir"
                    className="console-input"
                    {...register('output_dir')}
                  />
                </div>
              )}
            </section>

            <section className="data-card form-card">
              <div className="section-heading-row compact-row">
                <div>
                  <p className="section-eyebrow">Digest workflow</p>
                  <h2 className="section-title">日报与文章发布设置</h2>
                  <p className="section-description">
                    这里决定每天几点生成日报、是否自动跑任务，以及单篇文章是否进入发布流程。
                  </p>
                </div>
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="publish-digest-time">
                  日报生成时间
                </label>
                <input
                  id="publish-digest-time"
                  className="console-input"
                  type="time"
                  {...register('digest_schedule_time')}
                />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="publish-digest-enabled">
                  启用自动日报
                </label>
                <input
                  id="publish-digest-enabled"
                  type="checkbox"
                  {...register('digest_enabled')}
                />
              </div>

              <ChoiceCardGroup
                name="publish-article-mode"
                label="文章发布流程"
                value={articlePublishMode}
                options={[...articlePublishModeOptions]}
                onChange={(nextMode) => {
                  setValue('article_publish_mode', nextMode, {
                    shouldDirty: true,
                    shouldTouch: true,
                  });
                }}
              />
              <input type="hidden" {...register('article_publish_mode')} />

              <ChoiceCardGroup
                name="publish-article-review"
                label="文章发布审核"
                value={articleReviewMode}
                options={[...articleReviewModeOptions]}
                onChange={(nextMode) => {
                  setValue('article_review_mode', nextMode, {
                    shouldDirty: true,
                    shouldTouch: true,
                  });
                }}
              />
              <input type="hidden" {...register('article_review_mode')} />

              <div className="metric-meta compact-stack">
                <div className="status-row">
                  <span>时区</span>
                  <span className="detail-value monospace">
                    {valueOrDash(schedulerConfig?.timezone ?? 'Asia/Shanghai')}
                  </span>
                </div>
                <div className="status-row">
                  <span>自动日报</span>
                  <StatusBadge status={digestEnabled ? 'ok' : 'disabled'} />
                </div>
              </div>
            </section>

            <section className="data-card soft-card">
              <div className="section-heading-row compact-row">
                <div>
                  <p className="section-eyebrow">Output policy</p>
                  <h2 className="section-title">输出策略</h2>
                </div>
              </div>
              <div className="metric-meta compact-stack">
                <div className="status-row">
                  <span>当前 Provider</span>
                  <span className="detail-value">{valueOrDash(providerSummary)}</span>
                </div>
                <div className="status-row">
                  <span>接入状态</span>
                  <StatusBadge status={connectionStatus} />
                </div>
                <div className="status-row">
                  <span>审核策略</span>
                  <span className="detail-value">{reviewSummary}</span>
                </div>
                <div className="status-row">
                  <span>最近测试</span>
                  <span className="detail-value monospace">
                    {valueOrDash(integration?.last_test_at)}
                  </span>
                </div>
              </div>
              <ul className="helper-list">
                <li>{outputPolicySummary}</li>
                <li>
                  {provider === 'halo'
                    ? '`halo`：直接推送每日汇总到 Halo，并为单篇文章保留后续扩展能力。'
                    : '`markdown_export`：将日报或单篇内容导出为 Markdown，便于静态站或外部脚本消费。'}
                </li>
              </ul>
            </section>
          </form>
        ) : null}
      </div>
    </section>
  );
}
