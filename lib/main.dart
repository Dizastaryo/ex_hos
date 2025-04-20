import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:workmanager/workmanager.dart';
import 'package:path_provider/path_provider.dart';
// Providers
import 'providers/auth_provider.dart';

// Services
import 'services/product_service.dart';
import 'services/category_service.dart';
import 'services/order_service.dart';

// Screens
import 'screens/product_detail_screen.dart';
import 'screens/payment_screen.dart';
import 'screens/payment_status_screen.dart';
import 'screens/home_screen.dart';

import 'screens/my_orders_screen.dart';
import 'screens/splash_screen.dart';
import 'screens/auth_screen.dart';
import 'screens/about_screen.dart';
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

  // Настройка Dio и CookieJar
  final dio = Dio();
  final directory = await getApplicationDocumentsDirectory(); // Теперь работает
  final cookieJar = PersistCookieJar(
    storage: FileStorage("${directory.path}/.cookies/"),
  );
  dio.interceptors.add(CookieManager(cookieJar));

  // Исправленный вызов конструктора AuthProvider
  final authProvider =
      AuthProvider(dio, cookieJar); // Теперь принимает 2 аргумента

  // Настройка интерцепторов
  dio.interceptors.add(InterceptorsWrapper(
    onRequest: (options, handler) async {
      // Добавляем access token
      if (authProvider.token != null) {
        options.headers['Authorization'] = 'Bearer ${authProvider.token}';
      }
      return handler.next(options);
    },
    onError: (error, handler) async {
      if (error.response?.statusCode == 401 &&
          !error.requestOptions.extra.containsKey('retry')) {
        try {
          await authProvider.refreshToken();
          error.requestOptions.extra['retry'] = true;
          return handler.resolve(await dio.fetch(error.requestOptions));
        } catch (e) {
          return handler.next(error);
        }
      }
      return handler.next(error);
    },
  ));

  runApp(
    MultiProvider(
      providers: [
        Provider<Dio>.value(value: dio),
        Provider<CookieJar>.value(value: cookieJar),
        ChangeNotifierProvider<AuthProvider>.value(value: authProvider),
        Provider<ProductService>(create: (_) => ProductService(dio)),
        Provider<CategoryService>(create: (_) => CategoryService(dio)),
        Provider<OrderService>(create: (_) => OrderService(dio)),
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
        '/about': (_) => const AboutScreen(),
        '/product-detail': (context) => ProductDetailScreen(
              productId: ModalRoute.of(context)!.settings.arguments as int,
            ),
        '/orders': (context) => const MyOrdersScreen(),
        '/payment': (context) {
          final args = ModalRoute.of(context)!.settings.arguments
              as Map<String, dynamic>;
          return PaymentScreen(
            orderId: args['orderId'] as int,
            orderTotal: args['orderTotal'] as double,
          );
        },
        '/payment-status': (context) => PaymentStatusScreen(
              orderId: ModalRoute.of(context)!.settings.arguments as int,
            ),
      },
    );
  }
}
