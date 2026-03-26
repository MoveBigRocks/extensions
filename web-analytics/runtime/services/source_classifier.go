package analyticsservices

import analyticsdomain "github.com/movebigrocks/platform/extensions/web-analytics/runtime/domain"

func ClassifySource(utmSource, referrer, propertyDomain string) string {
	return analyticsdomain.ClassifySource(utmSource, referrer, propertyDomain)
}
