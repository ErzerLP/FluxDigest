import { Alert, Button, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';

import { PageHeader } from '../../../components/common/PageHeader';
import { SecretField } from '../../../components/forms/SecretField';
import { StatusBadge } from '../../../components/status/StatusBadge';
import {
  useSaveMinifluxConfig,
  useTestMinifluxConfig,
} from '../../../services/mutations/admin';
import { useAdminConfigs, useAdminStatus } from '../../../services/queries/admin';
import type { SecretInput, UpdateMinifluxConfigInput } from '../../../types/admin';

const defaultFetchLimit = 100;
const defaultLookbackHours = 24;

interface MinifluxConfigFormValues {
  base_url: string;
  fetch_limit: number;
  lookback_hours: number;
}

function normalizePositiveInt(value: number | undefined, fallback: number) {
  if (typeof value !== 'number' || !Number.isFinite(value) || value <= 0) {
    return fallback;
  }

  return Math.trunc(value);
}

function valueOrDash(value?: string) {
  return value?.trim() ? value : '—';
}

function isLoopbackHost(hostname: string) {
  const normalized = hostname.trim().toLowerCase();
  return (
    normalized === 'localhost' ||
    normalized === '0.0.0.0' ||
    normalized === '::1' ||
    normalized.startsWith('127.')
  );
}

function resolveMinifluxConsoleURL(baseURL?: string) {
  const trimmed = baseURL?.trim() ?? '';
  if (!trimmed) {
    return '';
  }

  try {
    const resolved = new URL(trimmed);
    const currentHostname = window.location.hostname.trim().toLowerCase();
    if (isLoopbackHost(resolved.hostname) && currentHostname && !isLoopbackHost(currentHostname)) {
      resolved.hostname = window.location.hostname;
    }
    const normalized = resolved.toString();
    if (!trimmed.endsWith('/') && resolved.pathname === '/' && !resolved.search && !resolved.hash) {
      return normalized.slice(0, -1);
    }
    return normalized;
  } catch {
    return trimmed;
  }
}

export function MinifluxConfigPage() {
  const configQuery = useAdminConfigs();
  const statusQuery = useAdminStatus();
  const saveMutation = useSaveMinifluxConfig();
  const testMutation = useTestMinifluxConfig();
  const [secretInput, setSecretInput] = useState<SecretInput>({ mode: 'keep' });
  const [saveGuidance, setSaveGuidance] = useState<string>();

  const currentConfig = configQuery.data?.miniflux;
  const configReady = configQuery.isSuccess;
  const integration = statusQuery.data?.integrations?.miniflux;
  const minifluxConsoleURL = resolveMinifluxConsoleURL(currentConfig?.base_url);

  const { register, handleSubmit, reset } = useForm<MinifluxConfigFormValues>({
    defaultValues: {
      base_url: '',
      fetch_limit: defaultFetchLimit,
      lookback_hours: defaultLookbackHours,
    },
  });

  useEffect(() => {
    if (!currentConfig) {
      return;
    }

    reset({
      base_url: currentConfig.base_url ?? '',
      fetch_limit: normalizePositiveInt(currentConfig.fetch_limit, defaultFetchLimit),
      lookback_hours: normalizePositiveInt(currentConfig.lookback_hours, defaultLookbackHours),
    });
    setSecretInput({ mode: currentConfig.api_token?.is_set ? 'keep' : 'replace', value: '' });
    setSaveGuidance(undefined);
  }, [currentConfig, reset]);

  const saveMessage = useMemo(() => {
    if (!saveMutation.isSuccess) {
      return null;
    }

    return 'Miniflux 配置已保存。';
  }, [saveMutation.isSuccess]);

  const onSubmit = async (values: MinifluxConfigFormValues) => {
    if (!configReady) {
      return;
    }

    if (secretInput.mode === 'replace' && !secretInput.value?.trim()) {
      setSaveGuidance('当前处于“替换密钥”模式，请先输入 Miniflux Token 再保存。');
      return;
    }

    const payload: UpdateMinifluxConfigInput = {
      base_url: values.base_url,
      fetch_limit: normalizePositiveInt(values.fetch_limit, defaultFetchLimit),
      lookback_hours: normalizePositiveInt(values.lookback_hours, defaultLookbackHours),
      api_token:
        secretInput.mode === 'replace'
          ? { mode: 'replace', value: secretInput.value ?? '' }
          : { mode: secretInput.mode },
    };

    setSaveGuidance(undefined);
    saveMutation.mutate(payload);
  };

  const connectionStatus = integration?.configured
    ? integration.last_test_status ?? 'configured'
    : 'missing';

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="Miniflux"
          subtitle="管理 Miniflux Reader 接入、抓取窗口与已保存配置的连通性测试。"
          actions={
            <div className="button-cluster">
              <Button
                onClick={() =>
                  window.open(minifluxConsoleURL, '_blank', 'noopener,noreferrer')
                }
                disabled={!configReady || !minifluxConsoleURL}
              >
                打开 Miniflux 后台
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
                loading={saveMutation.isPending}
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
            message="Miniflux 配置读取失败"
            description={configQuery.error instanceof Error ? configQuery.error.message : '未知错误'}
          />
        ) : null}

        {statusQuery.isError ? (
          <Alert
            type="warning"
            showIcon
            message="Miniflux 状态读取失败"
            description={statusQuery.error instanceof Error ? statusQuery.error.message : '未知错误'}
          />
        ) : null}

        {saveMessage ? <Alert type="success" showIcon message={saveMessage} /> : null}
        {saveMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="Miniflux 配置保存失败"
            description={saveMutation.error instanceof Error ? saveMutation.error.message : '未知错误'}
          />
        ) : null}
        {saveGuidance ? <Alert type="warning" showIcon message={saveGuidance} /> : null}
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
            message="Miniflux 连接测试失败"
            description={testMutation.error instanceof Error ? testMutation.error.message : '未知错误'}
          />
        ) : null}
        <Alert
          type="info"
          showIcon
          message="测试连接会校验当前已保存的 Miniflux 配置；如果修改了 Base URL 或 Token，请先保存后再测试。"
        />

        {configReady ? (
          <form
            className="config-form-grid"
            onSubmit={(event) => void handleSubmit(onSubmit)(event)}
          >
            <section className="data-card form-card">
              <div className="section-heading-row compact-row">
                <div>
                  <p className="section-eyebrow">Reader endpoint</p>
                  <h2 className="section-title">连接参数</h2>
                </div>
                <StatusBadge status={connectionStatus} />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="miniflux-base-url">
                  Base URL
                </label>
                <input id="miniflux-base-url" className="console-input" {...register('base_url')} />
              </div>

              <div className="form-grid-two">
                <div className="form-stack">
                  <label className="form-label" htmlFor="miniflux-fetch-limit">
                    Fetch Limit
                  </label>
                  <input
                    id="miniflux-fetch-limit"
                    type="number"
                    min={1}
                    step={1}
                    className="console-input"
                    {...register('fetch_limit', { valueAsNumber: true })}
                  />
                </div>
                <div className="form-stack">
                  <label className="form-label" htmlFor="miniflux-lookback-hours">
                    Lookback Hours
                  </label>
                  <input
                    id="miniflux-lookback-hours"
                    type="number"
                    min={1}
                    step={1}
                    className="console-input"
                    {...register('lookback_hours', { valueAsNumber: true })}
                  />
                </div>
              </div>

              <SecretField
                id="miniflux-api-token"
                label="Token"
                currentSecret={currentConfig?.api_token}
                value={secretInput}
                onChange={setSecretInput}
              />
            </section>

            <section className="data-card soft-card">
              <div className="section-heading-row compact-row">
                <div>
                  <p className="section-eyebrow">Runtime status</p>
                  <h2 className="section-title">同步与接入状态</h2>
                </div>
              </div>
              <div className="metric-meta compact-stack">
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
                <div className="status-row">
                  <span>推荐同步窗口</span>
                  <span className="detail-value">
                    {normalizePositiveInt(currentConfig?.lookback_hours, defaultLookbackHours)} 小时
                  </span>
                </div>
              </div>
              <p className="section-description">
                FluxDigest 只负责消费 Miniflux 已聚合的文章流；订阅源增删改仍建议在 Miniflux 原生后台中完成。
              </p>
            </section>
          </form>
        ) : null}
      </div>
    </section>
  );
}
