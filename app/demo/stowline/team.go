/*
|--------------------------------------------------------------------------
| The Support Team
|--------------------------------------------------------------------------
|
| Stowline's own support team: who they are, which team each sits on and
| what each can do. A small seed takes the first few, a large one the
| whole list, so the same people lead every size.
|
*/

package stowline

/*
Teammate is one person on Stowline's support team.
*/
type Teammate struct {
	Name, Email, Role, Team, Title string
}

/*
Teams are Stowline's support teams, with the inbox each one owns.
*/
func Teams() []struct{ Name, Inbox string } {
	return []struct{ Name, Inbox string }{
		{"Support", "Support"},
		{"Billing", "Billing"},
		{"Integrations", "Developers"},
		{"Enterprise", "Enterprise"},
	}
}

/*
Inboxes are where conversations land.
*/
func Inboxes() []string {
	return []string{"Support", "Billing", "Developers", "Enterprise"}
}

/*
Teammates returns the team, owner first, then the leads, then everyone
else; n caps how many.
*/
func Teammates(n int) []Teammate {
	all := []Teammate{
		{"Maya Lindgren", "maya", "owner", "Support", "Head of Support"},
		{"Kofi Asante", "kofi", "admin", "Support", "Support Lead"},
		{"Rosa Delgado", "rosa", "admin", "Billing", "Billing Lead"},
		{"Tomasz Nowak", "tomasz", "admin", "Integrations", "Support Engineer Lead"},
		{"Grace Whitfield", "grace", "member", "Enterprise", "Enterprise Support Manager"},
		{"Arjun Mehta", "arjun", "member", "Support", "Support Specialist"},
		{"Lena Vogel", "lena", "member", "Support", "Support Specialist"},
		{"Jamal Carter", "jamal", "member", "Integrations", "Support Engineer"},
		{"Aiko Morita", "aiko", "member", "Billing", "Billing Specialist"},
		{"Felipe Duarte", "felipe", "member", "Support", "Support Specialist"},
		{"Hannah Becker", "hannah", "member", "Support", "Support Specialist"},
		{"Chidi Okonkwo", "chidi", "member", "Integrations", "Support Engineer"},
		{"Sofia Marino", "sofia", "member", "Support", "Support Specialist"},
		{"Liam O'Connor", "liam", "member", "Enterprise", "Technical Account Manager"},
		{"Nadia Rahman", "nadia", "member", "Billing", "Billing Specialist"},
		{"Erik Johansson", "erik", "member", "Support", "Support Specialist"},
		{"Priya Raman", "priya", "member", "Integrations", "Support Engineer"},
		{"Lucas Fournier", "lucas", "member", "Support", "Support Specialist"},
		{"Mei Lin", "mei", "member", "Support", "Support Specialist"},
		{"Daniel Osei", "daniel", "member", "Enterprise", "Technical Account Manager"},
		{"Isabel Ruiz", "isabel", "member", "Support", "Support Specialist"},
		{"Noah Fischer", "noah", "member", "Integrations", "Support Engineer"},
		{"Zainab Bello", "zainab", "member", "Support", "Support Specialist"},
		{"Oskar Berg", "oskar", "member", "Billing", "Billing Specialist"},
		{"Chloe Martin", "chloe", "member", "Support", "Support Specialist"},
		{"Kenji Watanabe", "kenji", "member", "Integrations", "Support Engineer"},
		{"Amara Nwosu", "amara", "member", "Support", "Support Specialist"},
		{"Ruth Abebe", "ruth", "member", "Support", "Support Specialist"},
		{"Victor Hugo Lima", "victor", "member", "Enterprise", "Technical Account Manager"},
		{"Tess Morgan", "tess", "member", "Support", "Support Specialist"},
		{"Yusuf Demir", "yusuf", "member", "Integrations", "Support Engineer"},
		{"Ines Carvalho", "ines", "member", "Support", "Support Specialist"},
		{"Sam Whitaker", "sam", "member", "Billing", "Billing Specialist"},
		{"Olivia Brandt", "olivia", "member", "Support", "Support Specialist"},
		{"Rhys Llewellyn", "rhys", "member", "Support", "Support Specialist"},
		{"Wen Zhao", "wen", "member", "Integrations", "Support Engineer"},
		{"Fiona Gallagher", "fiona", "member", "Support", "Support Specialist"},
		{"Marco Bellini", "marco", "member", "Support", "Support Specialist"},
		{"Uma Krishnan", "uma", "member", "Enterprise", "Technical Account Manager"},
		{"Theo Laurent", "theo", "member", "Support", "Support Specialist"},
	}
	for i := range all {
		all[i].Email = all[i].Email + "@" + Domain
	}
	return all[:min(max(n, 5), len(all))]
}
