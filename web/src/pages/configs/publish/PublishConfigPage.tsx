import { Alert, Button, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';

import { PageHeader } from '../../../components/common/PageHeader';
import { SecretField } from '../../../components/forms/SecretField';
import { StatusBadge } from '../../../components/status/StatusBadge';
import {
  useSavePublishConfig,
  useTestPublishConfig,
} from '../../../services/mutations/admin';
import { useAdminConfigs, useAdminStatus } from '../../../services/queries/admin';
import type { PublishProvider, SecretInput, UpdatePublishConfigInput } from '../../../types/admin';

interface PublishConfigFormValues {
  provider: PublishProvider;
  halo_base_url: string;
  output_dir: string;
}

function valueOrDash(value?: string) {
  return value?.trim() ? value : '—';
}

export function PublishConfigPage() {
  const configQuery = useAdminConfigs();
  const statusQuery = useAdminStatus();
  const saveMutation = useSavePublishConfig();
  const testMutation = useTestPublishConfig();
  const [secretInput, setSecretInput] = useState<SecretInput>({ mode: 'keep' });
  const [saveGuidance, setSaveGuidance] = useState<string>();

  const currentConfig = configQuery.data?.publish;
  const configReady = configQuery.isSuccess;
  const integration = statusQuery.data?.integrations?.publisher;

  const { register, handleSubmit, reset, watch } = useForm<PublishConfigFormValues>({
    defaultValues: {
      provider: 'halo',
      halo_base_url: '',
      output_dir: '',
    },
  });

  const provider = watch('provider');

  useEffect(() => {
    if (!currentConfig) {
      return;
    }

    const nextProvider = currentConfig.provider === 'markdown_export' ? 'markdown_export' : 'halo';
    reset({
      provider: nextProvider,
      halo_base_url: currentConfig.halo_base_url ?? '',
      output_dir: currentConfig.output_dir ?? '',
    });
    setSecretInput({
      mode: nextProvider === 'halo' && !currentConfig.halo_token?.is_set ? 'replace' : 'keep',
      value: '',
    });
    setSaveGuidance(undefined);
  }, [currentConfig, reset]);

  const saveMessage = useMemo(() => {
    if (!saveMutation.isSuccess) {
      return null;
    }

    return '发布配置已保存。';
  }, [saveMutation.isSuccess]);

  const onSubmit = async (values: PublishConfigFormValues) => {
    if (!configReady) {
      return;
    }

    if (values.provider === 'halo' && secretInput.mode === 'replace' && !secretInput.value?.trim()) {
      setSaveGuidance('Halo 模式下当前处于“替换密钥”模式，请先输入 Halo Token 再保存。');
      return;
    }

    const payload: UpdatePublishConfigInput = {
      provider: values.provider,
      halo_base_url: values.halo_base_url,
      output_dir: values.output_dir,
      halo_token:
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
          title="Publish"
          subtitle="配置日报发布通道、输出策略与已保存渠道的连通性检查。"
          actions={
            <div className="button-cluster">
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
            message="发布配置读取失败"
            description={configQuery.error instanceof Error ? configQuery.error.message : '未知错误'}
          />
        ) : null}

        {saveMessage ? <Alert type="success" showIcon message={saveMessage} /> : null}
        {saveMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="发布配置保存失败"
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
                  <p className="section-eyebrow">Provider strategy</p>
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
                <li>`halo`：直接推送每日汇总到 Halo 站点。</li>
                <li>`markdown_export`：将日报导出为本地 Markdown，供后续静态发布或人工处理。</li>
              </ul>
            </section>
          </form>
        ) : null}
      </div>
    </section>
  );
}
