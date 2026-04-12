import { Alert, Button, Spin } from 'antd';
import { useEffect, useMemo, useState } from 'react';
import { useForm } from 'react-hook-form';

import { PageHeader } from '../../../components/common/PageHeader';
import { SecretField } from '../../../components/forms/SecretField';
import { StatusBadge } from '../../../components/status/StatusBadge';
import { useSaveLLMConfig, useTestLLMConfig } from '../../../services/mutations/admin';
import { useAdminConfigs } from '../../../services/queries/admin';
import type { LLMTestDraft, SecretInput, UpdateLLMConfigInput } from '../../../types/admin';

interface LLMConfigFormValues {
  base_url: string;
  model: string;
  is_enabled: boolean;
  timeout_ms: number;
}

export function LLMConfigPage() {
  const configQuery = useAdminConfigs();
  const saveMutation = useSaveLLMConfig();
  const testMutation = useTestLLMConfig();
  const [secretInput, setSecretInput] = useState<SecretInput>({ mode: 'keep' });

  const currentConfig = configQuery.data?.llm;

  const { register, handleSubmit, reset, getValues } = useForm<LLMConfigFormValues>({
    defaultValues: {
      base_url: '',
      model: '',
      is_enabled: true,
      timeout_ms: 30_000,
    },
  });

  useEffect(() => {
    if (!currentConfig) {
      return;
    }

    reset({
      base_url: currentConfig.base_url ?? '',
      model: currentConfig.model ?? '',
      is_enabled: currentConfig.is_enabled ?? true,
      timeout_ms: currentConfig.timeout_ms ?? 30_000,
    });
    setSecretInput({ mode: currentConfig.api_key?.is_set ? 'keep' : 'replace', value: '' });
  }, [currentConfig, reset]);

  const saveMessage = useMemo(() => {
    if (!saveMutation.isSuccess) {
      return null;
    }

    return '配置已保存。';
  }, [saveMutation.isSuccess]);

  const onSubmit = async (values: LLMConfigFormValues) => {
    const payload: UpdateLLMConfigInput = {
      base_url: values.base_url,
      model: values.model,
      is_enabled: values.is_enabled,
      timeout_ms: Number(values.timeout_ms),
      api_key:
        secretInput.mode === 'replace'
          ? { mode: 'replace', value: secretInput.value ?? '' }
          : { mode: secretInput.mode },
    };

    await saveMutation.mutateAsync(payload);
  };

  const handleTest = async () => {
    const values = getValues();
    const payload: LLMTestDraft = {
      base_url: values.base_url,
      model: values.model,
      timeout_ms: Number(values.timeout_ms),
      api_key: secretInput.mode === 'replace' ? secretInput.value : undefined,
    };

    await testMutation.mutateAsync(payload);
  };

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="LLM Config"
          subtitle="管理 base URL、模型与 API key 的最小可用配置入口。"
          actions={
            <div className="button-cluster">
              <Button onClick={() => void handleTest()} loading={testMutation.isPending}>
                测试连接
              </Button>
              <Button
                type="primary"
                onClick={() => void handleSubmit(onSubmit)()}
                loading={saveMutation.isPending}
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
            message="配置读取失败"
            description={configQuery.error instanceof Error ? configQuery.error.message : '未知错误'}
          />
        ) : null}

        {saveMessage ? <Alert type="success" showIcon message={saveMessage} /> : null}
        {saveMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="配置保存失败"
            description={saveMutation.error instanceof Error ? saveMutation.error.message : '未知错误'}
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

        <form className="config-form-grid" onSubmit={(event) => void handleSubmit(onSubmit)(event)}>
          <section className="data-card form-card">
            <div className="section-heading-row compact-row">
              <div>
                <p className="section-eyebrow">Endpoint</p>
                <h2 className="section-title">连接参数</h2>
              </div>
              <StatusBadge status={currentConfig?.api_key?.is_set ? 'configured' : 'missing'} />
            </div>

            <div className="form-stack">
              <label className="form-label" htmlFor="llm-base-url">
                Base URL
              </label>
              <input id="llm-base-url" className="console-input" {...register('base_url')} />
            </div>

            <div className="form-stack">
              <label className="form-label" htmlFor="llm-model">
                Model
              </label>
              <input id="llm-model" className="console-input" {...register('model')} />
            </div>

            <SecretField
              id="llm-api-key"
              label="API Key"
              currentSecret={currentConfig?.api_key}
              value={secretInput}
              onChange={setSecretInput}
            />
          </section>

          <section className="data-card form-card">
            <div className="section-heading-row compact-row">
              <div>
                <p className="section-eyebrow">Runtime</p>
                <h2 className="section-title">运行参数</h2>
              </div>
            </div>

            <div className="form-grid-two">
              <div className="form-stack">
                <label className="form-label" htmlFor="llm-timeout-ms">
                  Timeout (ms)
                </label>
                <input
                  id="llm-timeout-ms"
                  className="console-input"
                  type="number"
                  {...register('timeout_ms', { valueAsNumber: true })}
                />
              </div>

              <label className="toggle-card" htmlFor="llm-enabled">
                <span>
                  <span className="form-label">启用 LLM</span>
                  <span className="field-hint">关闭后任务仍保留，但不会调用模型。</span>
                </span>
                <input id="llm-enabled" type="checkbox" {...register('is_enabled')} />
              </label>
            </div>
          </section>
        </form>
      </div>
    </section>
  );
}
