import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import 'providers/auth_provider.dart';
import 'providers/box_provider.dart';
import 'screens/home_screen.dart';
import 'screens/auth_sms_code_screen.dart';

void main() {
  final dio = Dio();
  final cookieJar = CookieJar();
  dio.interceptors.add(CookieManager(cookieJar));

  runApp(
    MultiProvider(
      providers: [
        Provider<Dio>.value(value: dio),
        Provider<CookieJar>.value(value: cookieJar),
        ChangeNotifierProvider(create: (_) => AuthProvider(dio, cookieJar)),
        ChangeNotifierProvider(create: (_) => BoxProvider()),
      ],
      child: const MyApp(),
    ),
  );
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Склад для хранения вещей',
      theme: ThemeData(
        primaryColor: const Color(0xFF6C9942),
        colorScheme: ColorScheme.light(
          primary: const Color(0xFF6C9942),
          secondary: const Color(0xFF4A6E2B),
          background: const Color(0xFFDCEAD3),
        ),
        fontFamily: 'Montserrat',
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFF6C9942),
          foregroundColor: Colors.white,
          elevation: 5,
          centerTitle: true,
        ),
      ),
      initialRoute: '/auth',
      routes: {
        '/auth': (context) => const AuthSMSCodeScreen(),
        '/main': (context) => HomeScreen(),
      },
    );
  }
}
