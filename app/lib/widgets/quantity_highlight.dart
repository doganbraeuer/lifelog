import 'package:flutter/material.dart';

/// A number: digits, joined by `.`, `,`, `:` or `/` (`400`, `2,5`,
/// `1.000`, `12.11.2026`, `8:30`, `130/85`). Not inside a word, so the `1`
/// in `HbA1c` or the `12` in `B12` is no number of its own.
final _number = RegExp(r'(?<![\p{L}\p{N}])\d+(?:[.,:/]\d+)*', unicode: true);

/// A unit word right after a number or after `pro`: at most one space
/// before it, then a letter, `°` or `%`. Inside it may have digits, `/`, and
/// dots or hyphens between letters (`mg/dl`, `m²`, `µg`, `I.E`,
/// `Milli-Units`). A dot at the end of a sentence is not part of it.
final _word = RegExp(
  r'[   ]?([\p{L}°%][\p{L}\p{N}°%/]*(?:[.\-][\p{L}\p{N}°%/]+)*)',
  unicode: true,
);

/// `pro` or `je` between two unit words (`Milliunits pro Liter`).
final _per = RegExp(r'[   ](?:pro|je)(?![\p{L}\p{N}])', unicode: true);

/// Words that often follow a number but are no unit (`12.11.2026 nach dem
/// Termin`, `3 bis 5`, `130 zu 85`). After them only the number is marked.
const _notUnits = {
  'ab', 'aber', 'als', 'also', 'am', 'an', 'auch', 'auf', 'aus', 'bei', //
  'bis', 'bzw', 'ca', 'circa', 'danach', 'dann', 'das', 'dem', 'den', //
  'der', 'des', 'die', 'durch', 'ein', 'eine', 'einem', 'einen', 'einer', //
  'eines', 'er', 'es', 'etwa', 'für', 'gegen', 'hat', 'haben', 'hatte', //
  'ich', 'im', 'in', 'ist', 'jeweils', 'kein', 'keine', 'laut', 'mit', //
  'nach', 'nicht', 'noch', 'ohne', 'oder', 'seit', 'sie', 'sind', 'so', //
  'sowie', 'um', 'und', 'unter', 'über', 'vom', 'von', 'vor', 'war', //
  'waren', 'wie', 'wir', 'wird', 'werden', 'wurde', 'wurden', 'zu', 'zum', //
  'zur',
};

/// Finds numbers and the unit word right after them, e.g. `400 Milligramm`,
/// `2,5 Milliunits pro Liter`, `130/85 mmHg` or `37,6 °C`. The ranges are
/// sorted and do not overlap.
///
/// Speech recognition can write a real but wrong unit (`mL pro Liter`
/// instead of `Milliunits pro Liter`). No check catches that, only the
/// reader, so these places are marked for checking.
///
/// Generous on purpose: any word right after a number counts as its unit,
/// unless it is a common function word ([_notUnits]). With a list of known
/// units, an unusual or misspelled unit would stay unmarked, and that is
/// exactly the case that matters.
List<TextRange> findQuantities(String text) {
  final ranges = <TextRange>[];
  for (final number in _number.allMatches(text)) {
    // A digit inside the previous unit (`mg/2`) starts nothing new.
    if (ranges.isNotEmpty && number.start < ranges.last.end) continue;
    var end = number.end;
    final unitEnd = _unitEnd(text, end);
    if (unitEnd != null) {
      end = unitEnd;
      // „… pro Liter“ belongs to the unit as well.
      final per = _per.matchAsPrefix(text, end);
      final perEnd = per == null ? null : _unitEnd(text, per.end);
      if (perEnd != null) end = perEnd;
    }
    ranges.add(TextRange(start: number.start, end: end));
  }
  return ranges;
}

/// End of the unit word that starts at [start], or `null` if there is none.
/// `pro Tag` right after a number (`3 pro Tag`) counts as one unit.
int? _unitEnd(String text, int start) {
  final match = _word.matchAsPrefix(text, start);
  if (match == null) return null;
  final word = match.group(1)!.toLowerCase();
  if (_notUnits.contains(word)) return null;
  if (word == 'pro' || word == 'je') return _unitEnd(text, match.end);
  return match.end;
}

/// Text controller that marks numbers and units ([findQuantities]) while
/// [highlightQuantities] is on. The text itself stays plain: the marks are
/// only drawn, they are never part of [text].
class QuantityHighlightController extends TextEditingController {
  QuantityHighlightController({super.text});

  var _highlightQuantities = false;

  bool get highlightQuantities => _highlightQuantities;

  set highlightQuantities(bool value) {
    if (value == _highlightQuantities) return;
    _highlightQuantities = value;
    notifyListeners(); // the field draws itself again
  }

  @override
  TextSpan buildTextSpan({
    required BuildContext context,
    TextStyle? style,
    required bool withComposing,
  }) {
    if (!_highlightQuantities) {
      return super.buildTextSpan(
        context: context,
        style: style,
        withComposing: withComposing,
      );
    }
    final colors = Theme.of(context).colorScheme;
    return TextSpan(
      style: style,
      children: quantitySpans(
        text,
        findQuantities(text),
        // The word the keyboard is still working on, underlined as usual.
        composing: withComposing && value.isComposingRangeValid
            ? value.composing
            : TextRange.empty,
        marked: TextStyle(
          backgroundColor: colors.tertiaryContainer,
          color: colors.onTertiaryContainer,
          // Not by color alone: bold also shows up without color vision.
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

/// Splits [text] into spans: [marked] for the [quantities], an underline
/// for the [composing] range (like Flutter's own [TextEditingController]).
/// Together the spans give back [text] unchanged.
@visibleForTesting
List<TextSpan> quantitySpans(
  String text,
  List<TextRange> quantities, {
  TextRange composing = TextRange.empty,
  required TextStyle marked,
}) {
  const underline = TextStyle(decoration: TextDecoration.underline);
  final spans = <TextSpan>[];

  void add(int start, int end, TextStyle? style) {
    if (start >= end) return;
    if (!composing.isValid ||
        composing.isCollapsed ||
        composing.end <= start ||
        composing.start >= end) {
      spans.add(TextSpan(text: text.substring(start, end), style: style));
      return;
    }
    final from = composing.start.clamp(start, end);
    final to = composing.end.clamp(start, end);
    if (start < from) {
      spans.add(TextSpan(text: text.substring(start, from), style: style));
    }
    spans.add(
      TextSpan(
        text: text.substring(from, to),
        style: style?.merge(underline) ?? underline,
      ),
    );
    if (to < end) {
      spans.add(TextSpan(text: text.substring(to, end), style: style));
    }
  }

  var position = 0;
  for (final quantity in quantities) {
    add(position, quantity.start, null);
    add(quantity.start, quantity.end, marked);
    position = quantity.end;
  }
  add(position, text.length, null);
  return spans;
}
