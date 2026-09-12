// 列表分页统一口径（用户 2026-09-12 要求：设备台账每页 20 条，
// 其他查看类列表每页 10 条，减少视觉疲劳）。
//
// 用法：<Table pagination={listPagination()} />（需要 20 条时传 PAGE_SIZE_LEDGER）。
// hideOnSinglePage：数据不足一页时不显示分页控件，避免小字典表出现多余分页条。

/** 设备台账（总查看）每页条数。 */
export const PAGE_SIZE_LEDGER = 20;

/** 其他查看类列表每页条数。 */
export const PAGE_SIZE_LIST = 10;

export interface ListPagination {
  pageSize: number;
  showSizeChanger: boolean;
  hideOnSinglePage: boolean;
}

/** 生成统一的列表分页配置（默认每页 10 条）。 */
export function listPagination(pageSize: number = PAGE_SIZE_LIST): ListPagination {
  return { pageSize, showSizeChanger: false, hideOnSinglePage: true };
}
