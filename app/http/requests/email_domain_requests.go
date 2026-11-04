/*
|--------------------------------------------------------------------------
| Email Domain Requests
|--------------------------------------------------------------------------
|
| The inputs of the email domain routes. A domain is named by its public
| id in the path. Format asks for the records as a zone file instead.
|
*/

package requests

/*
EmailDomainRequest names one domain in the path.
*/
type EmailDomainRequest struct {
	Domain string `path:"domain"`
}

/*
ShowEmailDomainRequest is the input of GET /email-domains/{domain};
format bind answers with a zone file.
*/
type ShowEmailDomainRequest struct {
	Domain string `path:"domain"`
	Format string `query:"format" validate:"omitempty,oneof=json bind"`
}

/*
StoreEmailDomainRequest is the input of POST /email-domains.
*/
type StoreEmailDomainRequest struct {
	Body struct {
		Domain            string  `json:"domain"                        validate:"required,max=253"`
		MailFromSubdomain *string `json:"mail_from_subdomain,omitempty" validate:"omitempty,max=63"`
	}
}

/*
UpdateEmailDomainRequest is the input of PATCH /email-domains/{domain}.
*/
type UpdateEmailDomainRequest struct {
	Domain string `path:"domain"`
	Body   struct {
		MailFromSubdomain *string `json:"mail_from_subdomain,omitempty" validate:"omitempty,min=1,max=63"`
		DNSHost           *string `json:"dns_host,omitempty"            validate:"omitempty,max=32"`
	}
}

/*
DestroyEmailDomainRequest is the input of DELETE /email-domains/{domain}.
Force removes a domain addresses still send from.
*/
type DestroyEmailDomainRequest struct {
	Domain string `path:"domain"`
	Force  bool   `query:"force"`
}
