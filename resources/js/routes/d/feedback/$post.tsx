import { createFileRoute } from "@tanstack/react-router";

import { PostPage } from "@/features/feedback/post-page";

export const Route = createFileRoute("/d/feedback/$post")({
  component: function PostRoute() {
    const { post } = Route.useParams();

    return <PostPage key={post} id={post} />;
  },
});
