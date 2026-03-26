package analyticsservices

import analyticsdomain "github.com/movebigrocks/platform/extensions/web-analytics/runtime/domain"

func CountryFromLanguage(acceptLang string) string {
	return analyticsdomain.CountryFromLanguage(acceptLang)
}
