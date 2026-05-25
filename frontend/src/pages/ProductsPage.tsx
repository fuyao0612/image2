import { useState, useEffect } from 'react';
import { api } from '@/api/client';
import type { ProductItem, ProductCategoryItem } from '@/types/api';
import { useAppContext } from '@/contexts/AppContext';
import { Search, ShoppingBag } from 'lucide-react';
import { money } from '@/utils/helpers';
import { RechargeConfirmDialog } from "./RechargePage";
import { useBodyScrollLock } from '@/hooks/useBodyScrollLock';

export function ProductsPage() {
  const { token, setMessage } = useAppContext();
  
  const [products, setProducts] = useState<ProductItem[]>([]);
  const [productCategories, setProductCategories] = useState<ProductCategoryItem[]>([]);
  const [productKeyword, setProductKeyword] = useState('');
  const [activeProductCategoryId, setActiveProductCategoryId] = useState('');
  const [activeProduct, setActiveProduct] = useState<ProductItem | null>(null);

  const [rechargeConfirmDialog, setRechargeConfirmDialog] = useState<{
    open: boolean;
    title: string;
    lines: string[];
    confirmLabel: string;
    href?: string;
  }>({ open: false, title: '', lines: [], confirmLabel: '' });
  useBodyScrollLock(Boolean(activeProduct) || rechargeConfirmDialog.open)

  async function loadProductsPage() {
    try {
      const [productResult, categoryResult] = await Promise.all([
        api.listProducts(productKeyword, activeProductCategoryId),
        api.listProductCategories(),
      ]);
      setProducts(productResult.items ?? []);
      setProductCategories(categoryResult.items ?? []);
    } catch (error) {
      console.error(error);
    }
  }

  useEffect(() => {
    if (!token) return;
    void loadProductsPage();
  }, [token, productKeyword, activeProductCategoryId]);

  function handleProductPurchase(item: ProductItem) {
    if (!item.productUrl) {
      setMessage('该商品暂未配置购买链接，请稍后再试。');
      return;
    }
    setActiveProduct(null);
    setRechargeConfirmDialog({
      open: true,
      title: '确认前往购买',
      lines: [
        `您即将前往购买商品【${item.name}】`,
        '购买或开通完成后，相关权益将自动发放至您的账号。',
        '部分平台的权益可能需要您使用兑换码手动兑换。',
      ],
      confirmLabel: '前往购买',
      href: item.productUrl,
    });
  }

  function handleCancelRechargeConfirm() {
    setRechargeConfirmDialog({ open: false, title: '', lines: [], confirmLabel: '' });
  }

  function handleConfirmRechargeDialog() {
    if (rechargeConfirmDialog.href) {
      window.open(rechargeConfirmDialog.href, '_blank', 'noopener,noreferrer');
      setRechargeConfirmDialog({ open: false, title: '', lines: [], confirmLabel: '' });
      setMessage(rechargeConfirmDialog.title === '确认前往购买' ? '已打开商品购买页面。' : '已打开兑换码购买平台。购买完成后，请获取兑换码并返回本站兑换。');
    }
  }

  return (
    <>
      <ProductPage
        items={products}
        categories={productCategories}
        keyword={productKeyword}
        activeCategoryId={activeProductCategoryId}
        onKeywordChange={setProductKeyword}
        onCategoryChange={setActiveProductCategoryId}
        onOpenDetail={setActiveProduct}
      />
      {activeProduct ? (
        <ProductDetailDialog
          item={activeProduct}
          onClose={() => setActiveProduct(null)}
          onConfirm={() => handleProductPurchase(activeProduct)}
        />
      ) : null}
      {rechargeConfirmDialog.open ? (
        <RechargeConfirmDialog
          title={rechargeConfirmDialog.title}
          lines={rechargeConfirmDialog.lines}
          confirmLabel={rechargeConfirmDialog.confirmLabel}
          busy={false}
          onCancel={handleCancelRechargeConfirm}
          onConfirm={handleConfirmRechargeDialog}
        />
      ) : null}
    </>
  );
}
export function ProductPage({
  items,
  categories,
  keyword,
  activeCategoryId,
  onKeywordChange,
  onCategoryChange,
  onOpenDetail,
}: {
  items: ProductItem[]
  categories: ProductCategoryItem[]
  keyword: string
  activeCategoryId: string
  onKeywordChange: (value: string) => void
  onCategoryChange: (value: string) => void
  onOpenDetail: (item: ProductItem) => void
}) {
  return (
    <section className="space-y-4">
      <div className="rounded-2xl border border-slate-200 bg-white/95 p-3 shadow-sm dark:border-slate-800 dark:bg-slate-900/90">
        <div className="flex items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 px-3 py-2.5 dark:border-slate-700 dark:bg-slate-950/70">
          <Search className="h-4 w-4 text-slate-400" />
          <input
            value={keyword}
            onChange={(event) => onKeywordChange(event.target.value)}
            placeholder="搜索商品名、说明或分类"
            className="min-w-0 flex-1 bg-transparent text-sm text-slate-700 outline-none placeholder:text-slate-400 dark:text-slate-100"
          />
          {keyword ? (
            <button type="button" onClick={() => onKeywordChange('')} className="text-xs text-slate-400 transition hover:text-slate-600 dark:hover:text-slate-200">
              清空
            </button>
          ) : null}
        </div>
        <div className="mt-3 flex gap-2 overflow-x-auto pb-1">
          <button
            type="button"
            onClick={() => onCategoryChange('')}
            className={`shrink-0 rounded-full px-3 py-1.5 text-xs font-medium transition ${
              activeCategoryId ? 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300' : 'bg-amber-400 text-slate-900'
            }`}
          >
            全部
          </button>
          {categories.map((category) => (
            <button
              key={category.id}
              type="button"
              onClick={() => onCategoryChange(category.id)}
              className={`shrink-0 rounded-full px-3 py-1.5 text-xs font-medium transition ${
                activeCategoryId === category.id
                  ? 'bg-amber-400 text-slate-900'
                  : 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300'
              }`}
            >
              {category.name}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4 xl:grid-cols-5">
        {items.map((item) => {
          const imageUrl = item.previewUrl || item.previewThumbUrl
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onOpenDetail(item)}
              className="overflow-hidden rounded-lg border border-slate-200 bg-white text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-md dark:border-slate-800 dark:bg-slate-900"
            >
              <div className="aspect-square overflow-hidden bg-slate-100 dark:bg-slate-950">
                {imageUrl ? (
                  <img
                    src={imageUrl}
                    alt={item.name}
                    loading="lazy"
                    decoding="async"
                    className="h-full w-full object-cover"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-slate-400">
                    <ShoppingBag className="h-8 w-8" />
                  </div>
                )}
              </div>
              <div className="flex min-h-[92px] flex-col p-3">
                <h3 className="truncate text-[15px] font-semibold leading-5 text-slate-900 dark:text-slate-100">{item.name}</h3>
                {item.shortDescription ? (
                  <p className="mt-1 line-clamp-2 text-[12px] leading-4 text-slate-500 dark:text-slate-400">{item.shortDescription}</p>
                ) : (
                  <p className="mt-1 text-[12px] leading-4 text-slate-300 dark:text-slate-600"> </p>
                )}
                <div className="mt-auto flex justify-end pt-2 text-sm font-bold text-amber-600 dark:text-amber-300">{money(item.amountCents)}</div>
              </div>
            </button>
          )
        })}
      </div>
    </section>
  )
}
export function ProductDetailDialog({
  item,
  onClose,
  onConfirm,
}: {
  item: ProductItem
  onClose: () => void
  onConfirm: () => void
}) {
  const imageUrl = item.previewUrl || item.previewThumbUrl
  return (
    <div className="plain-modal-root fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 px-4 py-6" role="dialog" aria-modal="true">
      <button type="button" className="absolute inset-0" aria-label="关闭商品详情" onClick={onClose} />
      <div className="plain-modal-panel relative flex max-h-[84dvh] w-full max-w-[420px] flex-col overflow-hidden rounded-lg border border-slate-200 bg-white shadow-2xl dark:border-slate-700 dark:bg-slate-900">
        <div className="border-b border-slate-200 px-5 py-4 dark:border-slate-800">
          <h3 className="line-clamp-2 text-xl font-black tracking-tight text-slate-950 dark:text-slate-100">{item.name}</h3>
          {item.shortDescription ? (
            <p className="mt-1 text-sm leading-6 text-slate-500 dark:text-slate-400">{item.shortDescription}</p>
          ) : null}
          <div className="mt-2 text-lg font-black text-amber-600 dark:text-amber-300">{money(item.amountCents)}</div>
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-5">
          <div className="overflow-hidden rounded-lg bg-slate-100 dark:bg-slate-950">
            {imageUrl ? (
              <img src={imageUrl} alt={item.name} loading="lazy" decoding="async" className="aspect-square w-full object-cover" />
            ) : (
              <div className="flex aspect-square items-center justify-center bg-slate-100 text-slate-500 dark:bg-slate-950 dark:text-slate-500">
                <ShoppingBag className="h-12 w-12" />
              </div>
            )}
          </div>
          {item.categories.length ? (
            <div className="mt-3 flex flex-wrap gap-2">
              {item.categories.map((category) => (
                <span key={category.id} className="rounded-full bg-amber-300 px-2.5 py-1 text-xs font-medium text-amber-950">
                  {category.name}
                </span>
              ))}
            </div>
          ) : (
            <div className="mt-3 inline-flex rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-500 dark:bg-slate-800 dark:text-slate-300">未分类</div>
          )}
          {item.description ? (
            <div className="mt-4 whitespace-pre-wrap rounded-lg bg-slate-50 p-3 text-sm leading-6 text-slate-700 dark:bg-slate-950 dark:text-slate-200">
              {item.description}
            </div>
          ) : null}
        </div>
        <div className="grid grid-cols-2 border-t border-slate-200 dark:border-slate-800">
          <button type="button" onClick={onClose} className="h-12 border-r border-slate-200 text-sm font-medium text-slate-500 transition hover:bg-slate-50 dark:border-slate-800 dark:text-slate-300 dark:hover:bg-slate-950">
            取消
          </button>
          <button type="button" onClick={onConfirm} className="h-12 text-sm font-bold text-amber-600 transition hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-slate-950">
            {item.productUrl ? '购买' : '待配置'}
          </button>
        </div>
      </div>
    </div>
  )
}
