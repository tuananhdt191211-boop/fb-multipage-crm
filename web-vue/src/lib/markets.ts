// Thị trường bán hàng = quốc gia + ngôn ngữ + tiền tệ. Đổi thị trường là đổi cả ngôn ngữ và tiền.
// Ngôn ngữ phải khớp i18nLangNames ở backend (site_i18n.go).

export interface Market {
  code: string
  flag: string
  country: string // tên nước bằng tiếng nước đó
  lang: string
  currency: string
  locale: string // định dạng số/tiền, vd. de-DE
  region: 'Châu Á' | 'Châu Âu' | 'Bắc Mỹ & Châu Đại Dương' | 'Mỹ Latinh'
}

export const LANG_NAMES: Record<string, string> = {
  vi: 'Tiếng Việt', en: 'English', de: 'Deutsch', fr: 'Français', it: 'Italiano', es: 'Español',
  nl: 'Nederlands', pt: 'Português', pl: 'Polski', cs: 'Čeština', sk: 'Slovenčina', sv: 'Svenska',
  da: 'Dansk', fi: 'Suomi', nb: 'Norsk', el: 'Ελληνικά', hu: 'Magyar', ro: 'Română', bg: 'Български',
  hr: 'Hrvatski', sl: 'Slovenščina', et: 'Eesti', lv: 'Latviešu', lt: 'Lietuvių', uk: 'Українська'
}

// Tên tiếng Việt của ngôn ngữ (cho chủ shop đọc)
export const LANG_VI: Record<string, string> = {
  vi: 'Tiếng Việt', en: 'Tiếng Anh', de: 'Tiếng Đức', fr: 'Tiếng Pháp', it: 'Tiếng Ý', es: 'Tiếng Tây Ban Nha',
  nl: 'Tiếng Hà Lan', pt: 'Tiếng Bồ Đào Nha', pl: 'Tiếng Ba Lan', cs: 'Tiếng Séc', sk: 'Tiếng Slovakia', sv: 'Tiếng Thụy Điển',
  da: 'Tiếng Đan Mạch', fi: 'Tiếng Phần Lan', nb: 'Tiếng Na Uy', el: 'Tiếng Hy Lạp', hu: 'Tiếng Hungary', ro: 'Tiếng Romania',
  bg: 'Tiếng Bulgaria', hr: 'Tiếng Croatia', sl: 'Tiếng Slovenia', et: 'Tiếng Estonia', lv: 'Tiếng Latvia', lt: 'Tiếng Litva', uk: 'Tiếng Ukraina'
}

const m = (code: string, flag: string, country: string, lang: string, currency: string, locale: string, region: Market['region']): Market => ({
  code, flag, country, lang, currency, locale, region
})

export const MARKETS: Market[] = [
  m('vn', '🇻🇳', 'Việt Nam', 'vi', 'VND', 'vi-VN', 'Châu Á'),
  // Bắc Mỹ & Châu Đại Dương
  m('us', '🇺🇸', 'United States', 'en', 'USD', 'en-US', 'Bắc Mỹ & Châu Đại Dương'),
  m('ca-en', '🇨🇦', 'Canada (English)', 'en', 'CAD', 'en-CA', 'Bắc Mỹ & Châu Đại Dương'),
  m('ca-fr', '🇨🇦', 'Canada (Français)', 'fr', 'CAD', 'fr-CA', 'Bắc Mỹ & Châu Đại Dương'),
  m('au', '🇦🇺', 'Australia', 'en', 'AUD', 'en-AU', 'Bắc Mỹ & Châu Đại Dương'),
  m('nz', '🇳🇿', 'New Zealand', 'en', 'NZD', 'en-NZ', 'Bắc Mỹ & Châu Đại Dương'),
  // Châu Âu
  m('gb', '🇬🇧', 'United Kingdom', 'en', 'GBP', 'en-GB', 'Châu Âu'),
  m('ie', '🇮🇪', 'Ireland', 'en', 'EUR', 'en-IE', 'Châu Âu'),
  m('de', '🇩🇪', 'Deutschland', 'de', 'EUR', 'de-DE', 'Châu Âu'),
  m('at', '🇦🇹', 'Österreich', 'de', 'EUR', 'de-AT', 'Châu Âu'),
  m('ch', '🇨🇭', 'Schweiz', 'de', 'CHF', 'de-CH', 'Châu Âu'),
  m('fr', '🇫🇷', 'France', 'fr', 'EUR', 'fr-FR', 'Châu Âu'),
  m('be-fr', '🇧🇪', 'Belgique', 'fr', 'EUR', 'fr-BE', 'Châu Âu'),
  m('be-nl', '🇧🇪', 'België', 'nl', 'EUR', 'nl-BE', 'Châu Âu'),
  m('lu', '🇱🇺', 'Luxembourg', 'fr', 'EUR', 'fr-LU', 'Châu Âu'),
  m('nl', '🇳🇱', 'Nederland', 'nl', 'EUR', 'nl-NL', 'Châu Âu'),
  m('it', '🇮🇹', 'Italia', 'it', 'EUR', 'it-IT', 'Châu Âu'),
  m('es', '🇪🇸', 'España', 'es', 'EUR', 'es-ES', 'Châu Âu'),
  m('pt', '🇵🇹', 'Portugal', 'pt', 'EUR', 'pt-PT', 'Châu Âu'),
  m('pl', '🇵🇱', 'Polska', 'pl', 'PLN', 'pl-PL', 'Châu Âu'),
  m('cz', '🇨🇿', 'Česko', 'cs', 'CZK', 'cs-CZ', 'Châu Âu'),
  m('sk', '🇸🇰', 'Slovensko', 'sk', 'EUR', 'sk-SK', 'Châu Âu'),
  m('se', '🇸🇪', 'Sverige', 'sv', 'SEK', 'sv-SE', 'Châu Âu'),
  m('dk', '🇩🇰', 'Danmark', 'da', 'DKK', 'da-DK', 'Châu Âu'),
  m('no', '🇳🇴', 'Norge', 'nb', 'NOK', 'nb-NO', 'Châu Âu'),
  m('fi', '🇫🇮', 'Suomi', 'fi', 'EUR', 'fi-FI', 'Châu Âu'),
  m('gr', '🇬🇷', 'Ελλάδα', 'el', 'EUR', 'el-GR', 'Châu Âu'),
  m('hu', '🇭🇺', 'Magyarország', 'hu', 'HUF', 'hu-HU', 'Châu Âu'),
  m('ro', '🇷🇴', 'România', 'ro', 'RON', 'ro-RO', 'Châu Âu'),
  m('bg', '🇧🇬', 'България', 'bg', 'EUR', 'bg-BG', 'Châu Âu'),
  m('hr', '🇭🇷', 'Hrvatska', 'hr', 'EUR', 'hr-HR', 'Châu Âu'),
  m('si', '🇸🇮', 'Slovenija', 'sl', 'EUR', 'sl-SI', 'Châu Âu'),
  m('ee', '🇪🇪', 'Eesti', 'et', 'EUR', 'et-EE', 'Châu Âu'),
  m('lv', '🇱🇻', 'Latvija', 'lv', 'EUR', 'lv-LV', 'Châu Âu'),
  m('lt', '🇱🇹', 'Lietuva', 'lt', 'EUR', 'lt-LT', 'Châu Âu'),
  m('ua', '🇺🇦', 'Україна', 'uk', 'UAH', 'uk-UA', 'Châu Âu'),
  // Mỹ Latinh
  m('mx', '🇲🇽', 'México', 'es', 'MXN', 'es-MX', 'Mỹ Latinh'),
  m('br', '🇧🇷', 'Brasil', 'pt', 'BRL', 'pt-BR', 'Mỹ Latinh'),
  m('ar', '🇦🇷', 'Argentina', 'es', 'ARS', 'es-AR', 'Mỹ Latinh'),
  m('cl', '🇨🇱', 'Chile', 'es', 'CLP', 'es-CL', 'Mỹ Latinh'),
  m('co', '🇨🇴', 'Colombia', 'es', 'COP', 'es-CO', 'Mỹ Latinh'),
  m('pe', '🇵🇪', 'Perú', 'es', 'PEN', 'es-PE', 'Mỹ Latinh'),
  m('uy', '🇺🇾', 'Uruguay', 'es', 'UYU', 'es-UY', 'Mỹ Latinh'),
  m('cr', '🇨🇷', 'Costa Rica', 'es', 'CRC', 'es-CR', 'Mỹ Latinh'),
  m('do', '🇩🇴', 'República Dominicana', 'es', 'DOP', 'es-DO', 'Mỹ Latinh')
]

export const MARKET_BY_CODE = new Map(MARKETS.map((x) => [x.code, x]))

export const MARKET_REGIONS = (['Châu Á', 'Châu Âu', 'Bắc Mỹ & Châu Đại Dương', 'Mỹ Latinh'] as const).map((r) => ({
  region: r,
  markets: MARKETS.filter((x) => x.region === r)
}))

// Các ngôn ngữ cần dịch cho danh sách thị trường (trừ tiếng Việt)
export function langsOf(codes: string[]): string[] {
  const s = new Set<string>()
  for (const c of codes) {
    const mk = MARKET_BY_CODE.get(c)
    if (mk && mk.lang !== 'vi') s.add(mk.lang)
  }
  return [...s]
}
