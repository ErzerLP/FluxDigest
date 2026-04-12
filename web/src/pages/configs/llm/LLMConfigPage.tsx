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
}

export function LLMConfigPage() {
  const configQuery = useAdminConfigs();
  const saveMutation = useSaveLLMConfig();
  const testMutation = useTestLLMConfig();
  const [secretInput, setSecretInput] = useState<SecretInput>({ mode: 'keep' });
  const [testGuidance, setTestGuidance] = useState<string>();
  const [saveGuidance, setSaveGuidance] = useState<string>();

  const currentConfig = configQuery.data?.llm;
  const configReady = configQuery.isSuccess;

  const { register, handleSubmit, reset, getValues } = useForm<LLMConfigFormValues>({
    defaultValues: {
      base_url: '',
      model: '',
    },
  });

  useEffect(() => {
    if (!currentConfig) {
      return;
    }

    reset({
      base_url: currentConfig.base_url ?? '',
      model: currentConfig.model ?? '',
    });
    setSecretInput({ mode: currentConfig.api_key?.is_set ? 'keep' : 'replace', value: '' });
    setTestGuidance(undefined);
    setSaveGuidance(undefined);
  }, [currentConfig, reset]);

  const saveMessage = useMemo(() => {
    if (!saveMutation.isSuccess) {
      return null;
    }

    return '配置已保存。';
  }, [saveMutation.isSuccess]);

  const onSubmit = async (values: LLMConfigFormValues) => {
    if (!configReady) {
      return;
    }

    if (secretInput.mode === 'replace' && !secretInput.value?.trim()) {
      setSaveGuidance('替换密钥时必须输入 API key。');
      return;
    }

    const payload: UpdateLLMConfigInput = {
      base_url: values.base_url,
      model: values.model,
      api_key:
        secretInput.mode === 'replace'
          ? { mode: 'replace', value: secretInput.value ?? '' }
          : { mode: secretInput.mode },
    };

    setSaveGuidance(undefined);
    saveMutation.mutate(payload);
  };

  const handleTest = () => {
    if (!configReady) {
      return;
    }

    if (secretInput.mode !== 'replace' || !secretInput.value?.trim()) {
      setTestGuidance('测试连接需要切换为替换密钥并输入待测 key。');
      return;
    }

    setTestGuidance(undefined);
    const values = getValues();
    const payload: LLMTestDraft = {
      base_url: values.base_url,
      model: values.model,
      api_key: secretInput.value,
    };

    testMutation.mutate(payload);
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
              <Button onClick={handleTest} loading={testMutation.isPending} disabled={!configReady}>
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
        {saveGuidance ? <Alert type="warning" showIcon message={saveGuidance} /> : null}
        {testMutation.isSuccess ? (
          <Alert
            type="info"
            showIcon
            message={`连接测试：${testMutation.data.status ?? 'unknown'}`}
            description={testMutation.data.message ?? '已收到测试结果'}
          />
        ) : null}
        {testGuidance ? <Alert type="warning" showIcon message={testGuidance} /> : null}
        {testMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="连接测试失败"
            description={testMutation.error instanceof Error ? testMutation.error.message : '未知错误'}
          />
        ) : null}

        {configReady ? (
          <form
            className="config-form-grid config-form-grid-single"
            onSubmit={(event) => void handleSubmit(onSubmit)(event)}
          >
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
          </form>
        ) : null}
      </div>
    </section>
  );
}
