const ZERO_DECIMAL_CURRENCIES = new Set(["BIF", "CLP", "DJF", "GNF", "JPY", "KMF", "KRW", "MGA", "PYG", "RWF", "UGX", "VND", "VUV", "XAF", "XOF", "XPF"]);

export function formatMinorAmount(amount: number, currency: string) {
  const normalizedCurrency = currency.toUpperCase();
  const fractionDigits = ZERO_DECIMAL_CURRENCIES.has(normalizedCurrency) ? 0 : 2;
  const divisor = 10 ** fractionDigits;
  const majorAmount = amount / divisor;

  return `${normalizedCurrency} ${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: fractionDigits,
    maximumFractionDigits: fractionDigits,
  }).format(majorAmount)}`;
}
