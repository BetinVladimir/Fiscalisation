jest.mock('react-native',()=>({useWindowDimensions:()=>({width:390,height:844,fontScale:1})}));
// Authored only; global test execution gate remains closed.
import {BREAKPOINTS,getScreenSize} from '../src/ui/responsive';
import {breakpoints} from '../src/ui/tokens';
test('responsive layout uses canonical breakpoint object',()=>expect(BREAKPOINTS).toBe(breakpoints));
test.each([['sm','xs'],['md','sm'],['lg','md']] as const)('boundary %s changes from %s at canonical width',(boundary,before)=>{expect(getScreenSize(breakpoints[boundary]-1)).toBe(before);expect(getScreenSize(breakpoints[boundary])).toBe(boundary);});
