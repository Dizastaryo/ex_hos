import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:workmanager/workmanager.dart';

// Providers
import 'providers/auth_provider.dart';

// Services
import 'services/product_service.dart';
import 'services/category_service.dart';

// Screens
import 'screens/home_screen.dart';
import 'screens/splash_screen.dart';
import 'screens/auth_screen.dart';
import 'screens/my_cart_screen.dart';
import 'screens/notifications_screen.dart';
import 'screens/reset_password_screen.dart';
import 'screens/admin_home_screen.dart';
import 'screens/moderator_home_screen.dart';
import 'screens/products_screen.dart';
import 'screens/add_product_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Инициализация уведомлений и фоновых задач
  await _initNotifications();
  await _requestNotificationPermissions();
  Workmanager().initialize(_callbackDispatcher);
  Workmanager().registerPeriodicTask(
    'notify_rentals',
    'notify_rentals',
    frequency: const Duration(days: 1),
  );

  // Создаём Dio без базового URL
  final dio = Dio();
  final cookieJar = CookieJar();
  dio.interceptors.add(CookieManager(cookieJar));

  // AuthProvider с единственным Dio
  final authProvider = AuthProvider(dio);

  // Интерцептор для вставки Bearer-токена и обновления
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) async {
        final token = authProvider.token;
        if (token?.isNotEmpty == true) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        return handler.next(options);
      },
      onError: (error, handler) async {
        if (error.response?.statusCode == 401 &&
            !error.requestOptions.extra.containsKey('retry')) {
          try {
            await authProvider.refreshToken();
            final newToken = authProvider.token;
            if (newToken != null) {
              final req = error.requestOptions;
              req.headers['Authorization'] = 'Bearer $newToken';
              req.extra['retry'] = true;
              final response = await dio.fetch(req);
              return handler.resolve(response);
            }
          } catch (_) {
            await authProvider.logout();
          }
        }
        return handler.next(error);
      },
    ),
  );

  runApp(
    MultiProvider(
      providers: [
        Provider<Dio>.value(value: dio),
        Provider<CookieJar>.value(value: cookieJar),
        ChangeNotifierProvider<AuthProvider>.value(value: authProvider),
        Provider<ProductService>(create: (_) => ProductService(dio)),
        // Убираем dio из вызова:
        Provider<CategoryService>(create: (_) => CategoryService(dio)),
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
      initialRoute: '/splash',
      routes: {
        '/splash': (_) => SplashScreen(),
        '/auth': (_) => const AuthScreen(),
        '/main': (_) => const HomeScreen(),
        '/products': (_) => const ProductsScreen(),
        '/add-product': (_) => const AddProductScreen(),
        '/my-cart': (_) => const MyCartScreen(),
        '/notifications': (_) => const NotificationsScreen(),
        '/reset-password': (_) => const ResetPasswordScreen(),
        '/admin-home': (_) => const AdminHomeScreen(),
        '/moderator-home': (_) => const ModeratorHomeScreen(),
      },
    );
  }
}
