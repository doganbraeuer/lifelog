import 'dart:async';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/answer.dart';
import '../model/citations.dart';
import '../model/entry.dart';
import '../model/source_marks.dart';
import '../module/module_catalog.dart';
import '../widgets/motion.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import '../widgets/skeleton.dart';
import 'original_location_screen.dart';

/// Maximum length of a question according to docs/API.md.
const maxQuestionLength = 500;

/// Height of the box with the full text of an expanded source.
const sourceTextHeight = 240.0;

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

  /// Loaded entries of the sources by ID, so that each entry is loaded only
  /// once per answer, even if several sources point to it.
  final _entries = <String, Future<Entry>>{};

  /// The source cards of the current answer by number, for the references
  /// in the answer text.
  var _cards = <int, GlobalKey<_SourceCardState>>{};

  /// One tap recognizer per number that has a source.
  var _recognizers = <int, TapGestureRecognizer>{};

  @override
  void initState() {
    super.initState();
    // So that the send button becomes enabled or disabled while typing.
    _field.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _field.dispose();
    _disposeRecognizers();
    super.dispose();
  }

  void _disposeRecognizers() {
    for (final recognizer in _recognizers.values) {
      recognizer.dispose();
    }
    _recognizers = {};
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
      _setAnswer(null);
      _error = null;
      _searching = true;
    });
    try {
      final answer = await widget.api.ask(text, module: widget.moduleId);
      if (!mounted) return;
      setState(() => _setAnswer(answer));
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error);
    } finally {
      if (mounted) setState(() => _searching = false);
    }
  }

  /// A new answer gets new cards, recognizers and loaded entries.
  void _setAnswer(Answer? answer) {
    _answer = answer;
    _entries.clear();
    _disposeRecognizers();
    _cards = {
      for (final source in answer?.sources ?? const <Source>[])
        source.number: GlobalKey<_SourceCardState>(),
    };
    _recognizers = {
      for (final number in _cards.keys)
        number: TapGestureRecognizer()..onTap = () => _reveal(number),
    };
  }

  /// Loads an entry once. A failed attempt is forgotten, so that „Erneut
  /// versuchen“ loads again.
  Future<Entry> _loadEntry(String id) {
    final loaded = _entries[id];
    if (loaded != null) return loaded;
    final future = widget.api.getEntry(id);
    _entries[id] = future;
    future.then(
      (_) {},
      onError: (Object _) {
        if (identical(_entries[id], future)) _entries.remove(id);
      },
    );
    return future;
  }

  /// A reference in the answer text was tapped.
  void _reveal(int number) => _cards[number]?.currentState?.reveal();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: const ModulesButton(),
        title: const Text('Fragen'),
        actions: [MockBanner(visible: widget.api.isMock)],
      ),
      // Not a ListView: it would not build the cards outside the screen,
      // and a reference in the answer text must be able to scroll to them.
      // The content is small (question, answer, a few sources).
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [_input(), const SizedBox(height: 16), ..._result()],
        ),
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
                // A selection area instead of SelectableText: the text stays
                // selectable and copyable (long press), and the references
                // still react to a short tap.
                child: SelectionArea(
                  child: Text.rich(
                    TextSpan(children: _answerSpans(answer, theme)),
                    style: theme.textTheme.bodyLarge,
                  ),
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
                key: _cards[source.number],
                source: source,
                sameEntry: [
                  for (final s in answer.sources)
                    if (s.entryId == source.entryId) s,
                ],
                answer: answer.answer,
                question: asked,
                loadEntry: _loadEntry,
                onOpen: () => widget.openEntry(source.entryId),
              ),
          ],
        ),
    ];
  }

  /// The answer text with every reference that has a source as a link. A
  /// reference without a source (e.g. [2] if only 1 and 3 came) stays plain
  /// text.
  List<TextSpan> _answerSpans(Answer answer, ThemeData theme) {
    final text = answer.answer;
    final primary = theme.colorScheme.primary;
    final link = TextStyle(
      color: primary,
      fontWeight: FontWeight.w600,
      decoration: TextDecoration.underline,
      decorationColor: primary.withValues(alpha: 0.5),
    );
    final spans = <TextSpan>[];
    var at = 0;
    for (final citation in findCitations(text)) {
      final recognizer = _recognizers[citation.number];
      if (recognizer == null) continue;
      if (citation.start > at) {
        spans.add(TextSpan(text: text.substring(at, citation.start)));
      }
      spans.add(
        TextSpan(
          text: text.substring(citation.start, citation.end),
          style: link,
          recognizer: recognizer,
          semanticsLabel: 'Quelle ${citation.number} anzeigen',
        ),
      );
      at = citation.end;
    }
    if (at < text.length) spans.add(TextSpan(text: text.substring(at)));
    return spans;
  }
}

/// A source in one compact line: number as in the answer text, type, date
/// and issuer. Expanded, it shows the full text of the entry with the lines
/// marked that match the answer sentence citing it ([markSources]).
///
/// Two separate tap targets, so that one is not hit instead of the other:
/// the left part opens the entry, the arrow button on the right (behind a
/// divider) shows or hides the text.
class _SourceCard extends StatefulWidget {
  const _SourceCard({
    super.key,
    required this.source,
    required this.sameEntry,
    required this.answer,
    required this.question,
    required this.loadEntry,
    required this.onOpen,
  });

  final Source source;

  /// All sources of the answer from the same entry, this one included. Their
  /// lines are marked too, those of this one more strongly.
  final List<Source> sameEntry;

  /// Answer text and question; they decide which lines are marked.
  final String answer;
  final String question;

  /// Loads the entry (once per answer, see the question screen).
  final Future<Entry> Function(String entryId) loadEntry;
  final VoidCallback onOpen;

  @override
  State<_SourceCard> createState() => _SourceCardState();
}

/// The full text with the marks of the sources from this entry.
typedef _MarkedText = ({String fullText, SourceMarks marks});

class _SourceCardState extends State<_SourceCard> {
  /// `true` while the text is shown. Every card starts collapsed.
  var _expanded = false;

  /// `true` while the entry is loading.
  var _loading = false;

  /// The full text with the places, once loaded and found. Computed only
  /// once: the text view scrolls again for every new location object.
  _MarkedText? _marked;

  /// The entry could not be loaded; the excerpt is shown instead.
  ApiException? _loadError;

  /// `true` for a short time after a reference in the answer was tapped.
  var _highlighted = false;
  Timer? _highlightTimer;

  /// After a tap on a reference: once the card is fully expanded, scroll
  /// again so that its end is visible as well.
  var _scrollWhenExpanded = false;

  @override
  void dispose() {
    _highlightTimer?.cancel();
    super.dispose();
  }

  void _toggle() {
    setState(() => _expanded = !_expanded);
    if (_expanded && _marked == null && !_loading && _loadError == null) {
      _load();
    }
  }

  void _load() {
    setState(() {
      _loading = true;
      _loadError = null;
    });
    final source = widget.source;
    widget
        .loadEntry(source.entryId)
        .then(
          (entry) {
            if (!mounted) return;
            setState(() {
              _loading = false;
              _marked = _mark(entry.fullText);
            });
          },
          onError: (Object error) {
            if (!mounted) return;
            setState(() {
              _loading = false;
              _loadError = error is ApiException
                  ? error
                  : const ApiException(
                      code: ErrorCode.unexpectedResponse,
                      message: 'Unerwartete Antwort des Servers.',
                    );
            });
          },
        );
  }

  /// Finds the lines of this source and of the others from the same entry.
  /// `null` if the passage of this source is not found; then only the
  /// excerpt is shown.
  _MarkedText? _mark(String fullText) {
    final marks = markSources(
      fullText: fullText,
      source: widget.source,
      sameEntry: widget.sameEntry,
      answer: widget.answer,
      question: widget.question,
    );
    return marks == null ? null : (fullText: fullText, marks: marks);
  }

  /// A reference in the answer text points to this card: expand it, scroll
  /// to it and highlight it briefly.
  void reveal() {
    if (!_expanded) {
      _toggle();
      _scrollWhenExpanded = !motionDisabled(context);
    }
    _highlightTimer?.cancel();
    setState(() => _highlighted = true);
    _highlightTimer = Timer(Durations.extralong4, () {
      if (mounted) setState(() => _highlighted = false);
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      Scrollable.ensureVisible(
        context,
        alignment: 0.1,
        duration: motionDuration(context, Durations.medium4),
        curve: Easing.standard,
      );
    });
  }

  void _expansionEnded() {
    if (!_scrollWhenExpanded || !mounted) return;
    _scrollWhenExpanded = false;
    Scrollable.ensureVisible(
      context,
      alignmentPolicy: ScrollPositionAlignmentPolicy.keepVisibleAtEnd,
      duration: motionDuration(context, Durations.medium2),
      curve: Easing.standard,
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final catalog = ModuleCatalog.of(context);
    final source = widget.source;
    // Type name from the module of the source; a type is only unique with it.
    final typeName = catalog.typeName(source.module, source.type);
    final date = source.documentDate?.display ?? 'ohne Datum';
    final issuer = source.issuer ?? 'Quelle unbekannt';
    // Always the number from the backend, never the position in the list:
    // the numbers can have gaps (e.g. only 1 and 3).
    final number = source.number;
    final cardColor =
        CardTheme.of(context).color ?? theme.colorScheme.surfaceContainerLow;
    // Highlighted at once, then it fades back slowly. Not primaryContainer:
    // number circle and text marks have that color and would vanish.
    return TweenAnimationBuilder<Color?>(
      tween: ColorTween(
        end: _highlighted ? theme.colorScheme.secondaryContainer : cardColor,
      ),
      duration: motionDuration(
        context,
        _highlighted ? Durations.short2 : Durations.extralong4,
      ),
      curve: Easing.standard,
      builder: (context, color, child) =>
          Card(color: color, clipBehavior: Clip.antiAlias, child: child),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                // One label for the screen reader instead of the single
                // texts; the hint says what a double tap does.
                child: Semantics(
                  container: true,
                  button: true,
                  label: 'Quelle $number: $typeName, $date, $issuer',
                  onTapHint: 'Eintrag öffnen',
                  onTap: widget.onOpen,
                  excludeSemantics: true,
                  child: InkWell(
                    onTap: widget.onOpen,
                    child: Padding(
                      padding: const EdgeInsets.fromLTRB(12, 10, 8, 10),
                      child: Row(
                        children: [
                          CircleAvatar(radius: 16, child: Text('$number')),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    Icon(
                                      catalog.typeIcon(
                                        source.module,
                                        source.type,
                                      ),
                                      size: 16,
                                    ),
                                    const SizedBox(width: 4),
                                    Expanded(
                                      child: Text(
                                        '$typeName · $date',
                                        style: theme.textTheme.labelLarge,
                                      ),
                                    ),
                                  ],
                                ),
                                Text(
                                  issuer,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: theme.textTheme.bodySmall,
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 32, child: VerticalDivider(width: 1)),
              // The label names the source, otherwise a screen reader would
              // read several buttons with the same text.
              Semantics(
                button: true,
                expanded: _expanded,
                label: _expanded
                    ? 'Textstelle von Quelle $number ausblenden'
                    : 'Textstelle von Quelle $number anzeigen',
                onTap: _toggle,
                excludeSemantics: true,
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 4),
                  child: IconButton(
                    onPressed: _toggle,
                    tooltip: _expanded
                        ? 'Textstelle ausblenden'
                        : 'Textstelle anzeigen',
                    icon: Icon(
                      _expanded ? Icons.expand_less : Icons.expand_more,
                    ),
                  ),
                ),
              ),
            ],
          ),
          // The height glides along instead of the sources below jumping.
          AnimatedSize(
            duration: motionDuration(context, Durations.medium2),
            curve: Easing.emphasizedDecelerate,
            alignment: Alignment.topCenter,
            onEnd: _expansionEnded,
            child: _expanded
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      const Divider(height: 1),
                      // Own node for the screen reader, otherwise the text
                      // would be merged into the label of the arrow button.
                      Semantics(container: true, child: _details(theme)),
                    ],
                  )
                : const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }

  /// Below the line: loading, the full text with the place marked, or the
  /// excerpt (place not found, or the entry could not be loaded).
  Widget _details(ThemeData theme) {
    if (_loading) {
      return const SizedBox(
        height: sourceTextHeight,
        child: Center(
          child: CircularProgressIndicator(semanticsLabel: 'Text wird geladen'),
        ),
      );
    }
    final marked = _marked;
    if (marked != null) {
      final marks = marked.marks;
      // Without own marks, scrollTo is the passage: not in locations, so it
      // is not highlighted, but the view scrolls to its start.
      return SizedBox(
        height: sourceTextHeight,
        child: TextLocationView(
          fullText: marked.fullText,
          locations: [...marks.current, ...marks.others],
          current: marks.scrollTo,
          strongLocations: marks.current,
        ),
      );
    }
    final error = _loadError;
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (error != null) _loadErrorRow(theme, error),
          Text(
            '„${widget.source.excerpt}“',
            style: theme.textTheme.bodyMedium?.copyWith(
              fontStyle: FontStyle.italic,
            ),
          ),
        ],
      ),
    );
  }

  /// A calm note instead of an error card: the excerpt below still shows
  /// what the answer is based on.
  Widget _loadErrorRow(ThemeData theme, ApiException error) {
    // A deleted entry does not come back; trying again makes no sense.
    final gone =
        error.code == ErrorCode.entryNotFound ||
        error.code == ErrorCode.invalidId;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Expanded(
            child: Text(
              gone
                  ? 'Der Eintrag ist nicht mehr vorhanden.'
                  : 'Der ganze Text kann gerade nicht geladen werden.',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          if (!gone)
            TextButton(onPressed: _load, child: const Text('Erneut versuchen')),
        ],
      ),
    );
  }
}
