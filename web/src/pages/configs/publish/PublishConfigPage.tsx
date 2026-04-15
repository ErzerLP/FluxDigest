import { Alert, Button, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';

import { PageHeader } from '../../../components/common/PageHeader';
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

  const { register, handleSubmit, reset, watch } = useForm<PublishConfigFormValues>({
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
                </div>
                <StatusBadge status={connectionStatus} />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="publish-provider">
                  Provider
                </label>
                <select id="publish-provider" className="console-input" {...register('provider')}>
                  <option value="halo">Halo</option>
                  <option value="markdown_export">Markdown Export</option>
                </select>
              </div>

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

              <div className="form-stack">
                <label className="form-label" htmlFor="publish-article-mode">
                  文章发布流程
                </label>
                <select
                  id="publish-article-mode"
                  className="console-input"
                  {...register('article_publish_mode')}
                >
                  <option value="digest_only">只发日报</option>
                  <option value="suggested">部分发送（仅建议稿）</option>
                  <option value="all">全部发送</option>
                </select>
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="publish-article-review">
                  文章发布审核
                </label>
                <select
                  id="publish-article-review"
                  className="console-input"
                  {...register('article_review_mode')}
                >
                  <option value="manual_review">人工审核</option>
                  <option value="auto_publish">自动发布</option>
                </select>
              </div>

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
                  <span className="detail-value monospace">{valueOrDash(provider)}</span>
                </div>
                <div className="status-row">
                  <span>接入状态</span>
                  <StatusBadge status={connectionStatus} />
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
