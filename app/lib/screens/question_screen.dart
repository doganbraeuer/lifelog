import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/answer.dart';
import '../module/module_catalog.dart';
import '../widgets/motion.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import '../widgets/skeleton.dart';

/// Maximum length of a question according to docs/API.md.
const maxQuestionLength = 500;

/// Ask a question about one's own documents. The answer comes from the
/// backend and is based only on the sources shown. The app shows answer
/// and sources unchanged and adds nothing.
class QuestionScreen extends StatefulWidget {
  const QuestionScreen({
    super.key,
    required this.api,
    required this.moduleId,
    required this.openEntry,
  });

  final ApiClient api;

  /// Module in which the question is asked (`module` for `POST /questions`).
  final String moduleId;

  /// Opens the entry of a source. „Zurück“ (back) leads here again.
  final Future<void> Function(String entryId) openEntry;

  @override
  State<QuestionScreen> createState() => _QuestionScreenState();
}

class _QuestionScreenState extends State<QuestionScreen> {
  final _field = TextEditingController();

  /// The question asked last; stays visible above the answer.
  String? _asked;
  Answer? _answer;
  ApiException? _error;
  var _searching = false;

  @override
  void initState() {
    super.initState();
    // So that the send button becomes enabled or disabled while typing.
    _field.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _field.dispose();
    super.dispose();
  }

  bool get _canSend => !_searching && _field.text.trim().isNotEmpty;

  Future<void> _ask([String? question]) async {
    final text = (question ?? _field.text).trim();
    if (text.isEmpty || _searching) return;
    // Close the keyboard. Unfocus the field itself, not the scope: a scope
    // would remember the field and give the focus back later.
    FocusManager.instance.primaryFocus?.unfocus();
    setState(() {
      _asked = text;
      _answer = null;
      _error = null;
      _searching = true;
    });
    try {
      final answer = await widget.api.ask(text, module: widget.moduleId);
      if (!mounted) return;
      setState(() => _answer = answer);
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error);
    } finally {
      if (mounted) setState(() => _searching = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: const ModulesButton(),
        title: const Text('Fragen'),
        actions: [MockBanner(visible: widget.api.isMock)],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [_input(), const SizedBox(height: 16), ..._result()],
      ),
    );
  }

  Widget _input() {
    return TextField(
      controller: _field,
      minLines: 1,
      maxLines: 4,
      maxLength: maxQuestionLength,
      textInputAction: TextInputAction.send,
      onSubmitted: (_) => _ask(),
      decoration: InputDecoration(
        labelText: 'Deine Frage',
        hintText: 'z. B. Welchen Hämoglobinwert hatte ich im Oktober?',
        border: const OutlineInputBorder(),
        suffixIcon: IconButton(
          icon: const Icon(Icons.send),
          tooltip: 'Frage senden',
          onPressed: _canSend ? _ask : null,
        ),
      ),
    );
  }

  List<Widget> _result() {
    final asked = _asked;
    if (asked == null) {
      return [
        Text(
          'Die Antwort stützt sich nur auf deine hochgeladenen Dokumente '
          'und nennt die Stellen, aus denen sie stammt.',
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ];
    }

    final theme = Theme.of(context);
    final error = _error;
    final answer = _answer;
    return [
      Text('„$asked“', style: theme.textTheme.titleMedium),
      const SizedBox(height: 12),
      if (_searching) ...[
        Text(
          'Suche in deinen Dokumenten …',
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 12),
        // Skeleton in the form of answer and sources.
        const AnswerSkeleton(),
      ],
      if (error != null)
        Card(
          color: theme.colorScheme.errorContainer,
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  error.message,
                  style: TextStyle(color: theme.colorScheme.onErrorContainer),
                ),
                Align(
                  alignment: Alignment.centerRight,
                  child: TextButton(
                    onPressed: () => _ask(asked),
                    child: const Text('Erneut versuchen'),
                  ),
                ),
              ],
            ),
          ),
        ),
      // Answer and sources fade in one after another; the text is complete at
      // once (no typewriter effect). New answer, new run.
      if (answer != null)
        StaggeredFadeIn(
          key: ObjectKey(answer),
          children: [
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  answer.answer,
                  style: theme.textTheme.bodyLarge,
                ),
              ),
            ),
            if (answer.sources.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(4, 20, 4, 8),
                child: Text(
                  'Quellen',
                  style: theme.textTheme.titleSmall?.copyWith(
                    color: theme.colorScheme.primary,
                  ),
                ),
              ),
            for (final source in answer.sources)
              _SourceCard(
                source: source,
                onOpen: () => widget.openEntry(source.entryId),
              ),
          ],
        ),
    ];
  }
}

/// A source: number as in the answer text, document and excerpt.
class _SourceCard extends StatelessWidget {
  const _SourceCard({required this.source, required this.onOpen});

  final Source source;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final catalog = ModuleCatalog.of(context);
    // Type name from the module of the source; a type is only unique with it.
    final header = [
      catalog.typeName(source.module, source.type),
      source.documentDate?.display ?? 'ohne Datum',
    ].join(' · ');
    return Card(
      child: InkWell(
        onTap: onOpen,
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              CircleAvatar(radius: 16, child: Text('${source.number}')),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Icon(
                          catalog.typeIcon(source.module, source.type),
                          size: 16,
                        ),
                        const SizedBox(width: 4),
                        Expanded(
                          child: Text(
                            header,
                            style: theme.textTheme.labelLarge,
                          ),
                        ),
                      ],
                    ),
                    Text(
                      source.issuer ?? 'Quelle unbekannt',
                      style: theme.textTheme.bodySmall,
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '„${source.excerpt}“',
                      style: theme.textTheme.bodyMedium?.copyWith(
                        fontStyle: FontStyle.italic,
                      ),
                    ),
                  ],
                ),
              ),
              const Icon(Icons.chevron_right),
            ],
          ),
        ),
      ),
    );
  }
}
