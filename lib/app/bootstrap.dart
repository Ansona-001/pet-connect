import 'package:flutter/material.dart';
import './app.dart';

Future<void> bootstrap() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 🔥 Future: add services here
  // await Firebase.initializeApp();
  // await dotenv.load();

  runApp(const App());
}
