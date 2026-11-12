import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../model/entry.dart';
import '../module/module_definition.dart';
import '../platform/file_chooser.dart';
import '../widgets/motion.dart';
import '../widgets/navigation.dart';
import 'entry_screen.dart';
import 'question_screen.dart';
import 'upload_screen.dart';
import 'timeline_screen.dart';

/// Shell of a module with the navigation bar at the bottom (timeline,
/// upload, questions). All modules use this shell; the „Bereiche“ screen
/// sets the color scheme when opening it.
///
/// The three pages sit in a `PageView` (swipe or navigation bar) and stay
/// alive, so that their state is kept.
class ModuleShell extends StatefulWidget {
  const ModuleShell({
    super.key,
    required this.module,
    required this.api,
    required this.fileChooser,
  });

  final ModuleDefinition module;
  final ApiClient api;
  final FileChooser fileChooser;

  @override
  State<ModuleShell> createState() => _ModuleShellState();
}

class _ModuleShellState extends State<ModuleShell> {
  /// Page that is selected in the navigation bar.
  var _index = 0;

  final _pages = PageController();

  /// Target while animating after a tap in the navigation bar. When jumping
  /// from page 0 to 2 the PageView also reports page 1 on the way; without
  /// this lock the bar would briefly show „Hochladen“.
  int? _animatingTo;

  /// Access to the timeline, to reload it and to open entries. Works even
  /// when another page is visible right now, because all pages stay alive
  /// (see [_KeepAlivePage]).
  final _timeline = GlobalKey<TimelineScreenState>();

  @override
  void dispose() {
    _pages.dispose();
    super.dispose();
  }

  /// Tap in the navigation bar: switch the bar at once, the page animates
  /// after it. [haptics]: short tap feedback, only when the user switches
  /// themselves (not on the automatic switch after an upload).
  Future<void> _showPage(int index, {bool haptics = true}) async {
    if (index == _index && _animatingTo == null) return;
    if (haptics && index != _index) HapticFeedback.selectionClick();
    setState(() => _index = index);
    if (motionDisabled(context)) {
      // Jump directly without animation (a duration of 0 is not allowed when
      // scrolling).
      _pages.jumpToPage(index);
      return;
    }
    _animatingTo = index;
    await _pages.animateToPage(
      index,
      duration: Durations.medium2,
      curve: Easing.emphasizedDecelerate,
    );
    if (_animatingTo == index) _animatingTo = null;
  }

  /// Page change by swiping (or at the end of an animation).
  void _pageChanged(int index) {
    if (_animatingTo != null && index != _animatingTo) return;
    if (index != _index) {
      HapticFeedback.selectionClick();
      setState(() => _index = index);
    }
  }

  /// Switches to the timeline and opens the entry there. That way „Zurück“
  /// (back) leads from the entry into the (reloaded) timeline.
  void _openEntry(String entryId) {
    _showPage(0, haptics: false);
    _timeline.currentState?.openEntry(entryId);
  }

  void _uploaded(Entry entry) {
    _timeline.currentState?.reload();
    _openEntry(entry.id);
  }

  /// Opens an entry on top of the current page without switching to the
  /// timeline. That way „Zurück“ leads back e.g. to the answer to a question.
  Future<void> _openEntryOnTop(String entryId) async {
    await openPage<void>(
      context,
      EntryScreen(api: widget.api, entryId: entryId),
    );
    // In case something was corrected there.
    _timeline.currentState?.reload();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      // Swiping left or right changes the page.
      body: PageView(
        controller: _pages,
        onPageChanged: _pageChanged,
        children: [
          _KeepAlivePage(
            child: TimelineScreen(
              key: _timeline,
              api: widget.api,
              moduleId: widget.module.id,
              onUpload: () => _showPage(1),
            ),
          ),
          _KeepAlivePage(
            child: UploadScreen(
              api: widget.api,
              fileChooser: widget.fileChooser,
              moduleId: widget.module.id,
              uploaded: _uploaded,
              openEntry: _openEntry,
            ),
          ),
          _KeepAlivePage(
            child: QuestionScreen(
              api: widget.api,
              moduleId: widget.module.id,
              openEntry: _openEntryOnTop,
            ),
          ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: _showPage,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.timeline_outlined),
            selectedIcon: Icon(Icons.timeline),
            label: 'Zeitleiste',
          ),
          NavigationDestination(
            icon: Icon(Icons.upload_file_outlined),
            selectedIcon: Icon(Icons.upload_file),
            label: 'Hochladen',
          ),
          NavigationDestination(
            icon: Icon(Icons.question_answer_outlined),
            selectedIcon: Icon(Icons.question_answer),
            label: 'Fragen',
          ),
        ],
      ),
    );
  }
}

/// Keeps a page of the PageView alive, even when it is not visible.
///
/// Without it the PageView would dispose of invisible pages: sorting,
/// scroll position, chosen file, running upload and last answer would be
/// lost, and `_timeline.currentState` would be `null`.
class _KeepAlivePage extends StatefulWidget {
  const _KeepAlivePage({required this.child});

  final Widget child;

  @override
  State<_KeepAlivePage> createState() => _KeepAlivePageState();
}

class _KeepAlivePageState extends State<_KeepAlivePage>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;

  @override
  Widget build(BuildContext context) {
    super.build(context); // Required by the mixin: reports "keep alive".
    return widget.child;
  }
}
