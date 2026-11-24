/*
 * The settings of an A/B split: two to four variants, each a path with
 * a name and a share of conversations, the shares adding up to 100. A
 * conversation keeps its variant every time it meets the step. Picking
 * a winner sends everyone down it once the workflow is published again.
 */
import { Input } from "@/components/base/input/input";
import { Action, Danger, Field } from "@/features/settings/controls";
import { Row, Section } from "@/features/settings/layout";

import { type EditorProps, Note } from "./field-parts";
import { Pick } from "./pick";
import {
  fewestVariants,
  mostVariants,
  sharesTotal,
  variantsOf,
  winnerOf,
  withNewVariant,
  withoutVariant,
  withVariantLabel,
  withVariantWeight,
  withWinner,
} from "./split";

const noWinner = "none";

export function SplitFields({ step, onEdit }: EditorProps) {
  const variants = variantsOf(step);
  const total = sharesTotal(step);
  const winner = winnerOf(step);

  return (
    <>
      <Section
        title="Variants"
        description="Each conversation is given one variant by its share, and keeps it every time it comes back here."
      >
        {variants.map((v, index) => (
          <Row
            key={v.id}
            label={
              <Field
                label={`Variant ${index + 1} name`}
                value={v.label}
                width="w-32"
                onChange={(label) => onEdit((current) => withVariantLabel(current, v.id, label.slice(0, 40)))}
              />
            }
          >
            <span className="flex items-center gap-2">
              <Input
                aria-label={`Variant ${index + 1} share`}
                size="small"
                type="number"
                value={String(v.weight)}
                onChange={(value) => onEdit((current) => withVariantWeight(current, v.id, Number(value) || 0))}
                className="w-20"
              />
              <span className="text-body-2-regular text-text-tertiary">%</span>
              {variants.length > fewestVariants ? (
                <Danger onPress={() => onEdit((current) => withoutVariant(current, v.id))}>Remove</Danger>
              ) : null}
            </span>
          </Row>
        ))}
        {total === 100 ? null : <Note>{`The shares add up to ${total}%. Make them add up to 100%.`}</Note>}
        {variants.length < mostVariants ? (
          <Row label="Another variant" description="Shares split evenly again.">
            <Action onPress={() => onEdit(withNewVariant)}>Add variant</Action>
          </Row>
        ) : (
          <Note>A split has 4 variants at most.</Note>
        )}
      </Section>
      <Section title="Winner" description="Everyone goes down the winner from the next publish on.">
        <Row label="Send everyone down">
          <Pick
            key={winner}
            label="Winner"
            choices={[[noWinner, "No winner yet"], ...variants.map((v): [string, string] => [v.id, v.label || v.id])]}
            value={winner || noWinner}
            onChange={(id) => onEdit((current) => withWinner(current, id === noWinner ? "" : id))}
          />
        </Row>
      </Section>
    </>
  );
}
