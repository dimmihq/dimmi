/*
 * A made-up customer for a preview, so a teammate can try a procedure
 * without picking a real person: a name, an email and an id drawn at
 * random in code, never from a model, and the attributes the teammate
 * sets. A simulation saved from the chat keeps the same customer.
 */
import type { SimPersona } from "@/lib/api/procedures";

const first = [
  "Ada",
  "Amara",
  "Ana",
  "Chen",
  "Daniel",
  "Elif",
  "Emma",
  "Farah",
  "Hiro",
  "Ines",
  "Jonas",
  "Kwame",
  "Lea",
  "Lucas",
  "Maya",
  "Mohammed",
  "Nadia",
  "Noah",
  "Olga",
  "Priya",
  "Rafael",
  "Sara",
  "Tomas",
  "Yuki",
  "Zara",
];

const last = [
  "Adeyemi",
  "Bauer",
  "Costa",
  "Dubois",
  "Fischer",
  "Garcia",
  "Haddad",
  "Ivanova",
  "Kim",
  "Kowalski",
  "Larsen",
  "Mendes",
  "Nakamura",
  "Okafor",
  "Patel",
  "Rossi",
  "Schmidt",
  "Silva",
  "Tanaka",
  "Weber",
];

function pick<T>(list: T[]): T {
  return list[Math.floor(Math.random() * list.length)] as T;
}

/*
 * madeUpPerson draws a name, an email and an id. The email uses a
 * reserved test domain, so it can never reach anyone.
 */
export function madeUpPerson(): Pick<SimPersona, "name" | "email" | "external_id"> {
  const given = pick(first);
  const family = pick(last);
  const tag = Math.random().toString(36).slice(2, 8);

  return {
    name: `${given} ${family}`,
    email: `${given}.${family}.${tag}@example.test`.toLowerCase(),
    external_id: `test_${tag}`,
  };
}
