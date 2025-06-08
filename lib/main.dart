import 'dart:async';
import 'dart:math';
import 'package:dio/dio.dart';
import 'package:dio/io.dart';
import 'dart:io'; // Для использования HttpClient и X509Certificate
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:workmanager/workmanager.dart';
import 'package:path_provider/path_provider.dart';
import 'package:flutter_dotenv/flutter_dotenv.dart';
import 'services/appointment_service.dart';
import 'services/doctor_room_service.dart';

// Providers
import 'providers/auth_provider.dart';

import 'services/chat_service.dart';

import 'screens/home_screen.dart';
import 'screens/main_screen.dart';
import 'screens/splash_screen.dart';
import 'screens/support_screen.dart';
import 'screens/auth_screen.dart';
import 'screens/notifications_screen.dart';
import 'screens/reset_password_screen.dart';
import 'screens/admin_home_screen.dart';
import 'screens/moderator_home_screen.dart';
import 'services/admin_service.dart';

/// Глобальное переопределение HttpClient для принятия самоподписанных сертификатов
class MyHttpOverrides extends HttpOverrides {
  @override
  HttpClient createHttpClient(SecurityContext? context) {
    final client = super.createHttpClient(context);
    client.badCertificateCallback =
        (X509Certificate cert, String host, int port) => true;
    return client;
  }
}

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await dotenv.load(fileName: '.env');
  // Применяем глобальное переопределение для HttpClient
  HttpOverrides.global = MyHttpOverrides();

  // Инициализация уведомлений и фоновых задач
  await _initNotifications();
  await _requestNotificationPermissions();
  Workmanager().initialize(_callbackDispatcher);
  Workmanager().registerPeriodicTask(
    'notify_rentals',
    'notify_rentals',
    frequency: const Duration(days: 1),
  );

  // Настройка Dio и CookieJar
  final dio = Dio();
  final directory = await getApplicationDocumentsDirectory();
  final cookieJar = PersistCookieJar(
    storage: FileStorage('${directory.path}/.cookies/'),
  );
  dio.interceptors.add(CookieManager(cookieJar));

  // Переопределяем HttpClient для Dio, чтобы игнорировать ошибки SSL
  dio.httpClientAdapter = IOHttpClientAdapter(
    createHttpClient: () {
      final client = HttpClient();
      client.badCertificateCallback =
          (X509Certificate cert, String host, int port) => true;
      return client;
    },
  );

  // Провайдер аутентификации
  final authProvider = AuthProvider(dio, cookieJar);

  // Интерсептор для добавления Access-token
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        final token = authProvider.token;
        if (token != null) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        handler.next(options);
      },
      onError: (err, handler) async {
        if (err.response?.statusCode == 401 &&
            err.response?.statusCode == 403 &&
            !err.requestOptions.extra.containsKey('retry')) {
          try {
            await authProvider.silentAutoLogin();
            err.requestOptions.extra['retry'] = true;
            final clonedReq = await dio.fetch(err.requestOptions);
            handler.resolve(clonedReq);
          } catch (_) {
            handler.next(err);
          }
        } else {
          handler.next(err);
        }
      },
    ),
  );

  runApp(
    MultiProvider(
      providers: [
        Provider<Dio>.value(value: dio),
        Provider<CookieJar>.value(value: cookieJar),
        ChangeNotifierProvider<AuthProvider>.value(value: authProvider),
        Provider<UserService>(create: (_) => UserService(dio)),
        Provider<ChatService>(create: (_) => ChatService(dio)),
        Provider<AppointmentService>(create: (_) => AppointmentService(dio)),
        Provider<DoctorRoomService>(create: (_) => DoctorRoomService(dio)),
      ],
      child: const MyApp(),
    ),
  );
}

final FlutterLocalNotificationsPlugin _notificationsPlugin =
    FlutterLocalNotificationsPlugin();

Future<void> _initNotifications() async {
  const androidSettings = AndroidInitializationSettings('app_icon');
  const iosSettings = DarwinInitializationSettings();
  const settings = InitializationSettings(
    android: androidSettings,
    iOS: iosSettings,
  );
  await _notificationsPlugin.initialize(settings);
}

Future<void> _requestNotificationPermissions() async {
  if (!await Permission.notification.isGranted) {
    await Permission.notification.request();
  }
}

void _callbackDispatcher() {
  Workmanager().executeTask((task, inputData) async {
    if (task == 'notify_rentals') {
      // TODO: logic for notifications
      return Future.value(true);
    }
    return Future.value(false);
  });
}

class MyApp extends StatelessWidget {
  const MyApp({Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Mag Service',
      theme: ThemeData(
        primaryColor: const Color(0xFF6C9942),
        colorScheme: const ColorScheme.light(
          primary: Color(0xFF6C9942),
          secondary: Color(0xFF4A6E2B),
        ),
        fontFamily: 'Montserrat',
        useMaterial3: true,
      ),

      // ✅ Добавлено: локализация
      supportedLocales: const [
        Locale('ru'), // Русский
        Locale('en'), // Английский
      ],
      localizationsDelegates: const [
        GlobalMaterialLocalizations.delegate,
        GlobalWidgetsLocalizations.delegate,
        GlobalCupertinoLocalizations.delegate,
      ],
      locale: const Locale('ru'), // По умолчанию — русский

      initialRoute: '/splash',
      onGenerateRoute: (settings) {
        Widget page;
        switch (settings.name) {
          case '/splash':
            page = const SplashScreen();
            break;
          case '/auth':
            page = const AuthScreen();
            break;
          case '/main':
            page = const HomeScreen();
            break;
          case '/notifications':
            page = const NotificationsScreen();
            break;
          case '/reset-password':
            page = const ResetPasswordScreen();
            break;
          case '/admin-home':
            page = const AdminHomeScreen();
            break;
          case '/moderator-home':
            page = const ModeratorHomeScreen();
            break;
          case '/support':
            page = SupportScreen();
            break;
          default:
            page = const SplashScreen();
        }
        return CircularRevealRoute(page: page);
      },
    );
  }
}

class CircularRevealRoute extends PageRouteBuilder {
  final Widget page;
  CircularRevealRoute({required this.page})
      : super(
          transitionDuration: const Duration(milliseconds: 700),
          pageBuilder: (context, animation, secondaryAnimation) => page,
          transitionsBuilder: (context, animation, secondaryAnimation, child) {
            return ClipOval(
              clipper: CircleRevealClipper(
                fraction: animation.value,
                centerOffset: Offset(
                  MediaQuery.of(context).size.width / 2,
                  MediaQuery.of(context).size.height / 2,
                ),
              ),
              child: child,
            );
          },
        );
}

class CircleRevealClipper extends CustomClipper<Rect> {
  final double fraction;
  final Offset centerOffset;

  CircleRevealClipper({required this.fraction, required this.centerOffset});

  @override
  Rect getClip(Size size) {
    final maxRadius = sqrt(size.width * size.width + size.height * size.height);
    final radius = maxRadius * fraction;
    return Rect.fromCircle(center: centerOffset, radius: radius);
  }

  @override
  bool shouldReclip(CircleRevealClipper old) {
    return fraction != old.fraction;
  }
}
