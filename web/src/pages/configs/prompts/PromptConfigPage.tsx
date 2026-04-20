import { Alert, Button, Spin } from 'antd';
import { useEffect, useMemo } from 'react';
import { useForm } from 'react-hook-form';

import { PageHeader } from '../../../components/common/PageHeader';
import { useSavePromptConfig } from '../../../services/mutations/admin';
import { useAdminConfigs } from '../../../services/queries/admin';
import type { UpdatePromptConfigInput } from '../../../types/admin';
import { defaultPromptTemplates } from './defaultPromptTemplates';

interface PromptConfigFormValues {
  target_language: string;
  translation_prompt: string;
  analysis_prompt: string;
  dossier_prompt: string;
  digest_prompt: string;
}

export function PromptConfigPage() {
  const configQuery = useAdminConfigs();
  const saveMutation = useSavePromptConfig();
  const currentConfig = configQuery.data?.prompts;
  const configReady = configQuery.isSuccess;

  const { register, handleSubmit, reset } = useForm<PromptConfigFormValues>({
    defaultValues: {
      target_language: '',
      translation_prompt: '',
      analysis_prompt: '',
      dossier_prompt: '',
      digest_prompt: '',
    },
  });

  useEffect(() => {
    if (!currentConfig) {
      return;
    }

    reset({
      target_language: currentConfig.target_language ?? '',
      translation_prompt: currentConfig.translation_prompt ?? '',
      analysis_prompt: currentConfig.analysis_prompt ?? '',
      dossier_prompt: currentConfig.dossier_prompt ?? '',
      digest_prompt: currentConfig.digest_prompt ?? '',
    });
  }, [currentConfig, reset]);

  const saveMessage = useMemo(() => {
    if (!saveMutation.isSuccess) {
      return null;
    }

    return '提示词配置已保存。';
  }, [saveMutation.isSuccess]);

  const onSubmit = async (values: PromptConfigFormValues) => {
    if (!configReady) {
      return;
    }

    const payload: UpdatePromptConfigInput = {
      target_language: values.target_language,
      translation_prompt: values.translation_prompt,
      analysis_prompt: values.analysis_prompt,
      dossier_prompt: values.dossier_prompt,
      digest_prompt: values.digest_prompt,
    };

    saveMutation.mutate(payload);
  };

  const handleRestoreDefaults = () => {
    reset({
      target_language: defaultPromptTemplates.target_language,
      translation_prompt: defaultPromptTemplates.translation_prompt,
      analysis_prompt: defaultPromptTemplates.analysis_prompt,
      dossier_prompt: defaultPromptTemplates.dossier_prompt,
      digest_prompt: defaultPromptTemplates.digest_prompt,
    });
  };

  return (
    <section className="page-stack">
      <div className="console-panel page-panel">
        <PageHeader
          eyebrow="Configuration"
          title="Prompts"
          subtitle="统一管理翻译、分析、dossier 与日报生成提示词，并支持一键恢复内置默认模板。"
          actions={
            <div className="button-cluster">
              <Button onClick={handleRestoreDefaults} disabled={!configReady}>
                恢复默认
              </Button>
              <Button
                type="primary"
                onClick={() => void handleSubmit(onSubmit)()}
                loading={saveMutation.isPending}
                disabled={!configReady}
              >
                保存提示词
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
            message="提示词配置读取失败"
            description={configQuery.error instanceof Error ? configQuery.error.message : '未知错误'}
          />
        ) : null}

        {saveMessage ? <Alert type="success" showIcon message={saveMessage} /> : null}
        {saveMutation.isError ? (
          <Alert
            type="error"
            showIcon
            message="提示词保存失败"
            description={saveMutation.error instanceof Error ? saveMutation.error.message : '未知错误'}
          />
        ) : null}
        <Alert
          type="info"
          showIcon
          message="恢复默认只会回填表单内容，需点击“保存提示词”后才会成为新的运行时版本。"
        />

        {configReady ? (
          <form
            className="config-form-grid config-form-grid-single"
            onSubmit={(event) => void handleSubmit(onSubmit)(event)}
          >
            <section className="data-card form-card">
              <div className="section-heading-row compact-row">
                <div>
                  <p className="section-eyebrow">Runtime prompt set</p>
                  <h2 className="section-title">Prompt 编辑器</h2>
                </div>
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="prompt-target-language">
                  Target Language
                </label>
                <input
                  id="prompt-target-language"
                  className="console-input"
                  {...register('target_language')}
                />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="prompt-translation">
                  Translation Prompt
                </label>
                <textarea
                  id="prompt-translation"
                  className="console-input console-textarea"
                  {...register('translation_prompt')}
                />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="prompt-analysis">
                  Analysis Prompt
                </label>
                <textarea
                  id="prompt-analysis"
                  className="console-input console-textarea"
                  {...register('analysis_prompt')}
                />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="prompt-dossier">
                  Dossier Prompt
                </label>
                <textarea
                  id="prompt-dossier"
                  className="console-input console-textarea"
                  {...register('dossier_prompt')}
                />
              </div>

              <div className="form-stack">
                <label className="form-label" htmlFor="prompt-digest">
                  Digest Prompt
                </label>
                <textarea
                  id="prompt-digest"
                  className="console-input console-textarea"
                  {...register('digest_prompt')}
                />
              </div>
            </section>
          </form>
        ) : null}
      </div>
    </section>
  );
}
