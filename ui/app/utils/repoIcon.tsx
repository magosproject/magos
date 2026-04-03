import {
  IconBrandBitbucket,
  IconBrandGit,
  IconBrandGithub,
  IconBrandGitlab,
} from "@tabler/icons-react";

export function repoIcon(host: string, size = 16) {
  if (host.includes("github.com")) return <IconBrandGithub size={size} color="gray" />;
  if (host.includes("gitlab.com") || host.includes("gitlab."))
    return <IconBrandGitlab size={size} color="gray" />;
  if (host.includes("bitbucket.org")) return <IconBrandBitbucket size={size} color="gray" />;
  return <IconBrandGit size={size} color="gray" />;
}
