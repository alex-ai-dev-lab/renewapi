/*
Copyright (C) 2026 RenewAPI 贡献者

本文件遵循 GNU Affero General Public License 第 3 版或后续版本。
本程序不提供任何担保；许可证全文见仓库 LICENSE。
*/
import { useCallback, useEffect, useState } from 'react';
import { API, showError } from '../../helpers';

export function useChannelTestPrompts() {
  const [prompts, setPrompts] = useState([]);
  const [loading, setLoading] = useState(true);
  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/channel-test-prompts');
      if (!response.data.success) throw new Error(response.data.message);
      setPrompts(response.data.data);
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  return { prompts, loading, refresh };
}
