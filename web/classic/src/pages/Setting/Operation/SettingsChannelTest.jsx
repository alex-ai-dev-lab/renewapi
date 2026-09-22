/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import React from 'react';
import { Button, Form, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import ChannelTestPrompts from './ChannelTestPrompts';

const defaults = {
  prompt: 'hi',
  max_tokens: 0,
  reasoning_effort: '',
  endpoint_type: '',
  stream_mode: 'auto',
  timeout_seconds: 0,
  auto_test_only_auto_disabled: false,
};

export default function SettingsChannelTest(props) {
  const { t } = useTranslation();
  const [busy, setBusy] = React.useState(false);
  let initial = defaults;
  try {
    initial = {
      ...defaults,
      ...JSON.parse(props.options.ChannelTestSetting || '{}'),
    };
  } catch {
    /* 旧配置格式错误时保留可编辑的默认值。 */
  }
  const save = async (values) => {
    setBusy(true);
    try {
      const response = await API.put('/api/option/', {
        key: 'ChannelTestSetting',
        value: JSON.stringify({ ...initial, ...values }),
      });
      if (!response.data.success) throw new Error(response.data.message);
      showSuccess(t('保存成功'));
      await props.refresh();
    } catch (error) {
      showError(error.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className='space-y-6'>
      <ChannelTestPrompts />
      <Typography.Title heading={5}>{t('渠道测试参数')}</Typography.Title>
      <Form
        key={props.options.ChannelTestSetting}
        initValues={initial}
        onSubmit={save}
      >
        <Form.TextArea
          field='prompt'
          label={t('旧全局回退提示词')}
          extraText={t('没有可用提示词配置时使用；防投毒不会覆盖提示词。')}
        />
        <Form.Switch
          field='auto_test_only_auto_disabled'
          label={t('自动测试仅探测自动禁用渠道')}
          extraText={t('不影响手动测试，仍遵循自动恢复时间窗与周期。')}
        />
        <Form.InputNumber
          field='max_tokens'
          label='Max Tokens'
          min={0}
          precision={0}
          extraText={t('0 表示使用模型默认值')}
        />
        <Form.Select
          field='reasoning_effort'
          label={t('推理强度')}
          optionList={['', 'low', 'medium', 'high'].map((value) => ({
            value,
            label: value || t('默认'),
          }))}
        />
        <Form.Select
          field='endpoint_type'
          label={t('端点')}
          optionList={[
            '',
            'openai',
            'openai-response',
            'openai-response-compact',
            'anthropic',
            'gemini',
            'jina-rerank',
            'image-generation',
            'embeddings',
          ].map((value) => ({ value, label: value || t('自动检测') }))}
        />
        <Form.Select
          field='stream_mode'
          label={t('流模式')}
          optionList={['auto', 'on', 'off'].map((value) => ({
            value,
            label: value,
          }))}
        />
        <Form.InputNumber
          field='timeout_seconds'
          label={t('超时秒数')}
          min={0}
          precision={0}
          extraText={t('0 表示使用全局超时')}
        />
        <Button htmlType='submit' loading={busy}>
          {t('保存')}
        </Button>
      </Form>
    </div>
  );
}
