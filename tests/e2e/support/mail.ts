/*
 * Reading the mail the server sent, from Mailpit, the way a person opens
 * their inbox: the newest mail to an address whose subject has a word in
 * it, then a link or the code it carries.
 */
const api = process.env.MAILPIT_API_URL ?? "http://localhost:18025";

interface Message {
  ID: string;
}

async function newest(to: string, subject: string, count = 1): Promise<string> {
  const query = encodeURIComponent(`to:${to} subject:"${subject}"`);

  for (let attempt = 0; attempt < 50; attempt++) {
    const found = (await (await fetch(`${api}/api/v1/search?query=${query}&limit=${count}`)).json()) as {
      messages: Message[];
    };
    const id = found.messages.length >= count ? found.messages[0]?.ID : undefined;
    if (id) {
      const message = (await (await fetch(`${api}/api/v1/message/${id}`)).json()) as { Text: string };
      return message.Text;
    }

    await new Promise((resolve) => setTimeout(resolve, 200));
  }

  throw new Error(`no mail to ${to} with "${subject}" in its subject`);
}

/*
 * mailedLink returns the first link with a fragment in the newest mail
 * to the address whose subject has the words, once count such mails
 * arrived.
 */
export async function mailedLink(to: string, subject: string, count = 1): Promise<string> {
  const link = (await newest(to, subject, count)).match(/https?:\/\/[^\s<>]+#[^\s<>]+/)?.[0];
  if (!link) {
    throw new Error(`the mail to ${to} with "${subject}" carries no link`);
  }

  return link;
}

/*
 * mailedCode returns the six-digit code of the newest mail to the
 * address whose subject has the words.
 */
export async function mailedCode(to: string, subject: string): Promise<string> {
  const code = (await newest(to, subject)).match(/\b(\d{3})-(\d{3})\b/);
  if (!code) {
    throw new Error(`the mail to ${to} with "${subject}" carries no code`);
  }

  return `${code[1]}${code[2]}`;
}

/*
 * mailedTexts returns the text of every mail to the address, newest
 * first, as it stands now.
 */
export async function mailedTexts(to: string): Promise<string[]> {
  const query = encodeURIComponent(`to:${to}`);
  const found = (await (await fetch(`${api}/api/v1/search?query=${query}&limit=50`)).json()) as {
    messages: Message[];
  };
  const texts: string[] = [];
  for (const message of found.messages) {
    const full = (await (await fetch(`${api}/api/v1/message/${message.ID}`)).json()) as { Text: string };
    texts.push(full.Text);
  }

  return texts;
}
