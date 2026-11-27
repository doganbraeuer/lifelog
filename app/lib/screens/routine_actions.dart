import 'package:flutter/material.dart';

// What the routine screens share on screen. Sending a log is in
// routines/log_occurrence.dart, without Flutter UI, so that the reminder
// buttons in the background can use it too.

/// How long a message with „Rückgängig“ stays: longer than Flutter's 4 s,
/// so that there is time to read it and tap (target group: older users).
const undoSnackBarDuration = Duration(seconds: 8);

/// Shows [message] at the bottom in place of the current message. With an
/// [action] (e.g. „Rückgängig“) it stays [undoSnackBarDuration], then goes
/// by itself. `persist: false` is needed for that: by default Flutter keeps
/// a snack bar with an action until it is tapped, over every page, and
/// later messages wait behind it.
void showRoutineSnackBar(
  BuildContext context,
  String message, {
  SnackBarAction? action,
}) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(
      action == null
          ? SnackBar(content: Text(message))
          : SnackBar(
              content: Text(message),
              action: action,
              persist: false,
              duration: undoSnackBarDuration,
            ),
    );
}

/// Large buttons for the main actions (target group: older users).
const largeButtonSize = Size(88, 56);

ButtonStyle largeFilled(BuildContext context) => FilledButton.styleFrom(
  minimumSize: largeButtonSize,
  textStyle: Theme.of(context).textTheme.titleMedium,
);

ButtonStyle largeOutlined(BuildContext context) => OutlinedButton.styleFrom(
  minimumSize: largeButtonSize,
  textStyle: Theme.of(context).textTheme.titleMedium,
);
