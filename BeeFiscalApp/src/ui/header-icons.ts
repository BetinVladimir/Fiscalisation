import React from 'react';
import {Image} from 'react-native';
// Exact MaterialCommunityIcons images reused from Company POS; see header asset provenance.
const headerSources={
  'arrow-left':require('../../assets/header-icons/arrow-left.png'),
  'menu':require('../../assets/header-icons/menu.png'),
  'translate':require('../../assets/header-icons/translate.png'),
  'weather-night':require('../../assets/header-icons/weather-night.png'),
  'weather-sunny':require('../../assets/header-icons/weather-sunny.png'),
} as const;

// The enclosing button supplies the localized accessible name. Its bitmap is
// decorative, including on RN Web where Paper defaults image sources to role img.
export const headerIcons = Object.fromEntries(Object.entries(headerSources).map(([name,source]) => [name,
  ({size,color}:{size:number;color:string}) => React.createElement(Image, {
    source, accessible:false, accessibilityElementsHidden:true, importantForAccessibility:'no-hide-descendants',
    'aria-hidden':true, style:{width:size,height:size,tintColor:color},
  }),
])) as Record<keyof typeof headerSources,({size,color}:{size:number;color:string})=>React.ReactElement>;
