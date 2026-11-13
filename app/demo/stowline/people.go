/*
|--------------------------------------------------------------------------
| People and Companies
|--------------------------------------------------------------------------
|
| Stowline's customers are online stores, from one-person shops to
| brands with warehouses on two continents. Their names are built from
| hand-picked words the way shop names really are, and the people who
| write in come from the countries the carriers serve, with the odd one
| writing in their own language.
|
*/

package stowline

import (
	"fmt"
	"strings"
)

func firstNames() []string {
	return []string{
		"Ada", "Aaron", "Abena", "Aiko", "Alejandro", "Amara", "Andrea", "Anika", "Arjun", "Beatriz", "Ben", "Bianca",
		"Callum", "Camille", "Chen", "Chloe", "Chidi", "Dana", "Daniel", "Daria", "Dev", "Elena", "Eli", "Emeka",
		"Emma", "Erik", "Esther", "Farah", "Felix", "Fiona", "Gabriel", "Grace", "Hana", "Hannah", "Hugo", "Ifeoma",
		"Imani", "Ines", "Isaac", "Jack", "Jade", "James", "Jana", "Joao", "Jonas", "Julia", "Kai", "Karin", "Kemi",
		"Kenji", "Laila", "Lars", "Leah", "Lena", "Liam", "Lucia", "Luis", "Maya", "Marco", "Maria", "Mateo", "Mei",
		"Mia", "Miguel", "Nadia", "Naledi", "Nina", "Noah", "Olivia", "Omar", "Oskar", "Owen", "Paula", "Pedro",
		"Priya", "Rafael", "Rania", "Rhys", "Rosa", "Ruth", "Sam", "Sara", "Sean", "Sofia", "Sven", "Tariq", "Tess",
		"Theo", "Tomas", "Uma", "Valentina", "Victor", "Wen", "Will", "Yara", "Yusuf", "Zainab", "Zoe",
	}
}

func lastNames() []string {
	return []string{
		"Abara", "Adeyemi", "Alvarez", "Andersson", "Bakker", "Bauer", "Bennett", "Brooks", "Campbell", "Carvalho",
		"Chen", "Cohen", "Costa", "Dang", "Davies", "Delgado", "Dubois", "Eriksen", "Evans", "Fischer", "Fitzgerald",
		"Garcia", "Gonzalez", "Gupta", "Haddad", "Hansen", "Hartmann", "Hughes", "Ibrahim", "Ito", "Jansen", "Johnson",
		"Kaur", "Keller", "Kim", "Kowalski", "Kruger", "Larsen", "Laurent", "Lee", "Lopez", "Mahlangu", "Martin",
		"Mendes", "Mensah", "Moreau", "Murphy", "Nakamura", "Nguyen", "Novak", "Nwosu", "Obi", "Okafor", "Olsen",
		"Owusu", "Patel", "Pereira", "Petrov", "Quinn", "Reyes", "Rossi", "Russo", "Sato", "Schmidt", "Silva",
		"Singh", "Sorensen", "Tanaka", "Taylor", "Thompson", "Torres", "Van Dijk", "Vasquez", "Wagner", "Walsh",
		"Watanabe", "Weber", "Williams", "Wong", "Yilmaz", "Young", "Zhang",
	}
}

func shopWords() []string {
	return []string{
		"Juniper", "Fernway", "Alder", "Briar", "Cobalt", "Copper", "Driftwood", "Ember", "Field", "Fox", "Gilded",
		"Harbor", "Hazel", "Honey", "Indigo", "Ivy", "Kettle", "Lark", "Linden", "Lumen", "Maple", "Meadow", "Moss",
		"North", "Oak", "Olive", "Orchard", "Pebble", "Pine", "Quill", "Raven", "Saffron", "Sage", "Salt", "Sierra",
		"Slate", "Sparrow", "Summit", "Tallow", "Thistle", "Tide", "Timber", "Velvet", "Willow", "Wren", "Yarrow",
	}
}

func shopKinds() []string {
	return []string{
		"Coffee Co.", "Candles", "Apparel", "Ceramics", "Supply", "Goods", "Outfitters", "Botanicals", "Skincare",
		"Books", "Pet Co.", "Tea House", "Bike Parts", "Home", "Studio", "Leather", "Kitchen", "Toys", "Running",
		"Audio", "Print Shop", "Jewelry", "Wellness", "Garden",
	}
}

/*
Industries are what the stores sell, for their company attributes.
*/
func Industries() []string {
	return []string{
		"Apparel", "Beauty", "Food and drink", "Home goods", "Electronics", "Sports", "Books and media", "Pets",
		"Health", "Toys and games", "Jewelry", "Arts and crafts",
	}
}

/*
Place is where a customer is and how they write.
*/
type Place struct {
	Country, City, Timezone, Locale string
	Share                           float64
}

/*
Places returns where customers are, with the share of customers in
each.
*/
func Places() []Place {
	return []Place{
		{"US", "Austin", "America/Chicago", "en", 0.22},
		{"US", "Brooklyn", "America/New_York", "en", 0.14},
		{"US", "Portland", "America/Los_Angeles", "en", 0.10},
		{"CA", "Toronto", "America/Toronto", "en", 0.08},
		{"GB", "Manchester", "Europe/London", "en", 0.11},
		{"AU", "Melbourne", "Australia/Melbourne", "en", 0.07},
		{"DE", "Berlin", "Europe/Berlin", "de", 0.07},
		{"FR", "Lyon", "Europe/Paris", "fr", 0.06},
		{"ES", "Valencia", "Europe/Madrid", "es", 0.05},
		{"BR", "São Paulo", "America/Sao_Paulo", "pt", 0.04},
		{"NL", "Utrecht", "Europe/Amsterdam", "nl", 0.03},
		{"IE", "Dublin", "Europe/Dublin", "en", 0.03},
	}
}

/*
pickPlace draws a place by its share.
*/
func pickPlace(r *Random) Place {
	places := Places()
	roll := r.Float64()
	for _, p := range places {
		if roll < p.Share {
			return p
		}
		roll -= p.Share
	}
	return places[0]
}

/*
pickPlan draws a plan by the share of buyers on it.
*/
func pickPlan(r *Random) PricePlan {
	plans := Plans()
	roll := r.Float64()
	for _, p := range plans {
		if roll < p.ShareOfBuyers {
			return p
		}
		roll -= p.ShareOfBuyers
	}
	return plans[0]
}

/*
PersonName returns a full name.
*/
func PersonName(r *Random) string {
	return firstNames()[r.IntN(len(firstNames()))] + " " + lastNames()[r.IntN(len(lastNames()))]
}

/*
ShopName returns a store's name and its domain.
*/
func ShopName(r *Random, n int) (string, string) {
	word := shopWords()[r.IntN(len(shopWords()))]
	kind := shopKinds()[r.IntN(len(shopKinds()))]
	name := word + " " + kind
	if r.IntN(5) == 0 {
		name = word + " & " + shopWords()[r.IntN(len(shopWords()))]
	}
	slug := notSlug.ReplaceAllString(strings.ToLower(name), "")
	return name, fmt.Sprintf("%s%d.test", slug, n)
}

/*
emailOf builds a believable address for a person at a domain.
*/
func emailOf(r *Random, name, domain string, n int) string {
	parts := strings.Fields(strings.ToLower(name))
	first, last := parts[0], strings.Join(parts[1:], "")
	switch r.IntN(4) {
	case 0:
		return fmt.Sprintf("%s@%s", first, domain)
	case 1:
		return fmt.Sprintf("%s.%s@%s", first, last, domain)
	case 2:
		return fmt.Sprintf("%s%s@%s", first[:1], last, domain)
	default:
		return fmt.Sprintf("%s.%s%d@%s", first, last, n%97, domain)
	}
}
