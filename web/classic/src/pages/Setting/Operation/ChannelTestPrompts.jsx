/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import React, { useId, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Modal,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../helpers';
import { useChannelTestPrompts } from '../../../hooks/channel/useChannelTestPrompts';

export default function ChannelTestPrompts() {
  const { t } = useTranslation();
  const formId = useId();
  const { prompts, loading, refresh } = useChannelTestPrompts();
  const [editing, setEditing] = useState(undefined);
  const [deleting, setDeleting] = useState(null);
  const [busy, setBusy] = useState(false);
  const save = async (input, id) => {
    setBusy(true);
    try {
      const response = id
        ? await API.put(`/api/channel-test-prompts/${id}`, input)
        : await API.post('/api/channel-test-prompts', input);
      if (!response.data.success) throw new Error(response.data.message);
      setEditing(undefined);
      showSuccess(t('保存成功'));
      await refresh();
    } catch (error) {
      showError(error.message);
    } finally {
      setBusy(false);
    }
  };
  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    try {
      const response = await API.delete(
        `/api/channel-test-prompts/${deleting.id}`,
      );
      if (!response.data.success) throw new Error(response.data.message);
      setDeleting(null);
      await refresh();
    } catch (error) {
      showError(error.message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className='space-y-3'>
      <Typography.Title heading={5}>{t('测试提示词管理')}</Typography.Title>
      <Typography.Paragraph>
        {t('每个渠道可选择独立提示词，防投毒校验会追加 nonce。')}
      </Typography.Paragraph>
      <Button onClick={() => setEditing(null)} disabled={busy}>
        {t('新建提示词')}
      </Button>
      {!loading && prompts.length === 0 && (
        <Typography.Paragraph>
          {t('暂无提示词配置，使用旧全局提示词。')}
        </Typography.Paragraph>
      )}
      {prompts.map((prompt) => (
        <Card key={prompt.id} title={prompt.name}>
          <Space wrap>
            {prompt.is_default && <Tag>{t('默认')}</Tag>}
            {!prompt.enabled && <Tag>{t('已禁用')}</Tag>}
            <span>
              {t('引用渠道数')}: {prompt.reference_count}
            </span>
          </Space>
          <Typography.Paragraph>{prompt.description}</Typography.Paragraph>
          <Typography.Paragraph ellipsis={{ rows: 3 }}>
            {prompt.prompt}
          </Typography.Paragraph>
          <Space wrap>
            <Button disabled={busy} onClick={() => setEditing(prompt)}>
              {t('编辑')}
            </Button>
            <Button
              disabled={busy}
              onClick={() =>
                save({ ...prompt, enabled: !prompt.enabled }, prompt.id)
              }
            >
              {prompt.enabled ? t('禁用') : t('启用')}
            </Button>
            <Button
              disabled={busy || !prompt.enabled || prompt.is_default}
              onClick={() => save({ ...prompt, is_default: true }, prompt.id)}
            >
              {t('设为默认')}
            </Button>
            <Button
              type='danger'
              disabled={busy || prompt.reference_count > 0}
              onClick={() => setDeleting(prompt)}
            >
              {t('删除')}
            </Button>
          </Space>
        </Card>
      ))}
      <Modal
        visible={editing !== undefined}
        title={editing ? t('编辑提示词') : t('新建提示词')}
        footer={null}
        onCancel={() => setEditing(undefined)}
      >
        {editing !== undefined && (
          <Form
            key={editing?.id ?? 'new'}
            initValues={
              editing ?? {
                name: '',
                prompt: '',
                description: '',
                enabled: true,
                is_default: false,
              }
            }
            onSubmit={(values) => save(values, editing?.id)}
          >
            <Form.Input
              id={`${formId}-name`}
              field='name'
              label={t('名称')}
              maxLength={100}
              rules={[{ required: true }]}
            />
            <Form.TextArea
              id={`${formId}-prompt`}
              field='prompt'
              label={t('测试提示词')}
              maxLength={65536}
              rules={[{ required: true }]}
              autosize={{ minRows: 4, maxRows: 12 }}
            />
            <Form.Input
              id={`${formId}-description`}
              field='description'
              label={t('描述')}
              maxLength={2000}
            />
            <Form.Switch
              id={`${formId}-enabled`}
              field='enabled'
              label={t('启用')}
            />
            <Form.Switch
              id={`${formId}-default`}
              field='is_default'
              label={t('设为默认')}
              extraText={t('停用提示词会同时取消默认资格。')}
            />
            <Button htmlType='submit' loading={busy}>
              {t('保存')}
            </Button>
          </Form>
        )}
      </Modal>
      <Modal
        visible={!!deleting}
        title={t('删除提示词')}
        okText={t('删除')}
        cancelText={t('取消')}
        onCancel={() => setDeleting(null)}
        onOk={remove}
        confirmLoading={busy}
      >
        {deleting?.name}
      </Modal>
    </div>
  );
}
