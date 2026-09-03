import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import './app.dart';

Future<void> bootstrap() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 🔥 Future: add services here
  // await Firebase.initializeApp();
  // await dotenv.load();

  runApp(const ProviderScope(child: App()));
}
