/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/lib/format'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCreemPrice } from '../lib/format'
import type { CreemProduct } from '../types'

interface CreemProductsSectionProps {
  products: CreemProduct[]
  onProductSelect: (product: CreemProduct) => void
  loading?: boolean
}

export function CreemProductsSection({
  products,
  onProductSelect,
  loading,
}: CreemProductsSectionProps) {
  const { t } = useTranslation()

  if (loading) {
    return (
      <div className='grid min-w-0 gap-2'>
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className='h-24 rounded-lg' />
        ))}
      </div>
    )
  }

  if (!Array.isArray(products) || products.length === 0) {
    return null
  }

  return (
    <div className='grid min-w-0 gap-2'>
      {products.map((product) => (
        <Button
          key={product.productId}
          variant='outline'
          className='h-auto min-w-0 justify-between gap-3 rounded-md px-3 py-3 text-left whitespace-normal'
          onClick={() => onProductSelect(product)}
        >
          <span className='min-w-0'>
            <span className='block text-sm font-medium'>{product.name}</span>
            <span className='text-muted-foreground mt-1 block text-xs'>
              {t('Quota')}: {formatNumber(product.quota)}
            </span>
          </span>
          <span className='shrink-0 font-mono text-sm font-semibold tabular-nums'>
            {formatCreemPrice(product.price, product.currency)}
          </span>
        </Button>
      ))}
    </div>
  )
}
