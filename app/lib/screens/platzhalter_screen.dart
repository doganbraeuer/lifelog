import 'package:flutter/material.dart';

/// Vorläufiger Screen für Bereiche, die in einer späteren Etappe kommen.
class PlatzhalterScreen extends StatelessWidget {
  const PlatzhalterScreen({super.key, required this.titel});

  final String titel;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(titel)),
      body: const Center(child: Text('Kommt in einer der nächsten Etappen.')),
    );
  }
}
