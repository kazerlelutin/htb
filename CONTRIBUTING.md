# Contributing to HTB / Contribuer à HTB

Thanks for helping improve HTB. Small, focused contributions are easiest to
review and release.

Merci de contribuer à HTB. Les changements petits et ciblés sont les plus
simples à relire et à publier.

## Before you start / Avant de commencer

- Search existing GitHub issues and pull requests before opening a new one.
- Use an issue or an HTB ticket to describe the user-facing problem and its
  acceptance criteria before making a substantial change.
- Do not report security vulnerabilities in public issues; follow
  [SECURITY.md](SECURITY.md) instead.
- Keep each pull request to one coherent outcome. Discuss a large change in an
  issue first.

## Development workflow / Flux de contribution

1. Fork the repository and create a branch from `main`.
2. Make the smallest change that meets the agreed acceptance criteria.
3. Update documentation and Gherkin scenarios in `features/` when observable
   behaviour changes.
4. Add or update automated tests when code behaviour changes.
5. Run the local checks below before opening a pull request.

In the pull request, explain the problem and solution, link the issue or HTB
ticket, list the checks you ran, and call out migrations, compatibility
impacts, or follow-up work. Write user-visible changes in plain language; a
maintainer will add them to the appropriate section of [CHANGELOG.md](CHANGELOG.md)
before release.

For a documentation-only change, run the checks that apply and state why a
full test suite was unnecessary. Maintainers may ask for a smaller scope or a
follow-up issue rather than expanding an unrelated pull request.

## Local checks / Vérifications locales

```bash
go test ./...
go vet ./...
go build ./cmd/htbd ./cmd/htb
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./cmd/htbd ./cmd/htb
sh -n install.sh
```

The integration tests require PostgreSQL through
`HTBD_TEST_DATABASE_URL`. Use a dedicated test database only; never point this
variable at production. See [the self-hosting guide](docs/self-hosting.md) for
the development configuration.

Les tests d’intégration nécessitent PostgreSQL via
`HTBD_TEST_DATABASE_URL`. Utilisez uniquement une base dédiée aux tests, jamais
la production. Le [guide d’auto-hébergement](docs/self-hosting.fr.md) décrit
la configuration de développement.
