/*
 * Which readers an article is kept to: everyone, or one saved audience.
 * It saves at once, apart from the article's text, and applies to readers
 * and to dimmi alike, so dimmi never answers someone from an article
 * they could not read. Audiences that read what a browser could claim
 * cannot keep content; the server says so.
 */
import { useMutation, useQuery } from "@tanstack/react-query";

import { Select, SelectItem } from "@/components/base/select/select";
import { audiencesQuery } from "@/lib/api/audiences";
import { ApiError } from "@/lib/api/client";
import { type Article, setArticleAudience } from "@/lib/api/help-centers";

const everyone = "everyone";

function failure(error: unknown): string | null {
  if (!error) {
    return null;
  }
  const list = error instanceof ApiError ? (error.details.errors as { message: string }[] | undefined) : undefined;

  return list?.[0]?.message ?? (error instanceof Error ? error.message : "Not saved.");
}

export function ArticleAudience({
  workspace,
  article,
  onSaved,
}: {
  workspace: string;
  article: Article;
  onSaved: (next: Article) => void;
}) {
  const saved = useQuery({ ...audiencesQuery(workspace), enabled: workspace !== "" }).data ?? [];
  const save = useMutation({
    mutationFn: (audience: string | null) => setArticleAudience(workspace, article.id, audience),
    onSuccess: onSaved,
  });
  const choices: [string, string][] = [[everyone, "Everyone"], ...saved.map((a): [string, string] => [a.id, a.name])];
  const problem = failure(save.error);

  return (
    <div className="flex flex-col gap-1.5">
      <Select
        label="Audience"
        className="w-full"
        selectedKey={article.audience_id ?? everyone}
        onSelectionChange={(key) => {
          if (key !== null) {
            save.mutate(key === everyone ? null : String(key));
          }
        }}
      >
        {choices.map(([id, name]) => (
          <SelectItem key={id} id={id} textValue={name}>
            {name}
          </SelectItem>
        ))}
      </Select>
      {problem ? <span className="max-w-64 text-body-2-regular text-text-error-primary">{problem}</span> : null}
    </div>
  );
}
