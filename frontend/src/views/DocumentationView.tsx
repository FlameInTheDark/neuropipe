import { PageHeader } from '@/components/PageHeader'
import { DocumentationWorkspace } from '@/components/DocumentationWorkspace'
import { useTranslation } from 'react-i18next'

export function DocumentationView() {
  const { t } = useTranslation()
  return <section className="flex h-full min-h-0 flex-col"><PageHeader title={t('nav.documentation')} description={t('docs.description')} /><DocumentationWorkspace /></section>
}
